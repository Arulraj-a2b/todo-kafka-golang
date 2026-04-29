package cache

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"auth-service/internal/obs"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// Client wraps redis.Client with fail-open semantics. Methods on a nil/disabled
// client are no-ops so callers can fall back to the database.
type Client struct {
	rdb *redis.Client
}

func New() *Client {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if url == "" {
		slog.Info("REDIS_URL not set; cache disabled")
		return &Client{}
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		slog.Warn("invalid REDIS_URL; cache disabled", "url", url, "err", err)
		return &Client{}
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Warn("redis unreachable; cache disabled", "url", url, "err", err)
		return &Client{}
	}
	// Trace + record metrics for every redis call.
	if err := redisotel.InstrumentTracing(rdb); err != nil {
		slog.Warn("redisotel tracing instrument", "err", err)
	}
	if err := redisotel.InstrumentMetrics(rdb); err != nil {
		slog.Warn("redisotel metrics instrument", "err", err)
	}
	slog.Info("cache connected", "url", url)
	return &Client{rdb: rdb}
}

func (c *Client) Enabled() bool { return c != nil && c.rdb != nil }

// Get records a hit (success) or miss (redis.Nil) on the obs counters using
// the prefix before the first ':' as the label.
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	if !c.Enabled() {
		return nil, redis.Nil
	}
	v, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			obs.CacheMissesTotal.WithLabelValues(keyPrefix(key)).Inc()
			return nil, redis.Nil
		}
		return nil, err
	}
	obs.CacheHitsTotal.WithLabelValues(keyPrefix(key)).Inc()
	return v, nil
}

func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) {
	if !c.Enabled() {
		return
	}
	if err := c.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		slog.WarnContext(ctx, "cache set failed", "key", key, "err", err)
	}
}

func (c *Client) Del(ctx context.Context, keys ...string) {
	if !c.Enabled() || len(keys) == 0 {
		return
	}
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		slog.WarnContext(ctx, "cache del failed", "keys", keys, "err", err)
	}
}

func IsMiss(err error) bool { return errors.Is(err, redis.Nil) }

func keyPrefix(key string) string {
	if i := strings.Index(key, ":"); i > 0 {
		return key[:i]
	}
	return "unknown"
}
