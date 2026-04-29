package cache

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps redis.Client with fail-open semantics: every method tolerates
// a nil receiver (Redis disabled / unreachable) and turns into a no-op so the
// caller falls back to the database.
type Client struct {
	rdb *redis.Client
}

// New connects to REDIS_URL. Returns a Client whose methods no-op when Redis
// is unavailable so callers can keep going.
func New() *Client {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if url == "" {
		log.Println("REDIS_URL not set; cache disabled")
		return &Client{}
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("invalid REDIS_URL %q: %v; cache disabled", url, err)
		return &Client{}
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Printf("redis unreachable at %s: %v; cache disabled", url, err)
		return &Client{}
	}
	log.Printf("Cache connected to %s", url)
	return &Client{rdb: rdb}
}

func (c *Client) Enabled() bool { return c != nil && c.rdb != nil }

// Get returns the cached bytes or (nil, redis.Nil) on miss.
// On any error other than miss, returns (nil, error) so callers can decide.
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	if !c.Enabled() {
		return nil, redis.Nil
	}
	v, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, redis.Nil
		}
		return nil, err
	}
	return v, nil
}

// Set is best-effort. Errors are logged but not returned.
func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) {
	if !c.Enabled() {
		return
	}
	if err := c.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		log.Printf("cache set %s: %v", key, err)
	}
}

func (c *Client) Del(ctx context.Context, keys ...string) {
	if !c.Enabled() || len(keys) == 0 {
		return
	}
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		log.Printf("cache del %v: %v", keys, err)
	}
}

// IsMiss returns true when err is redis.Nil (cache miss, expected).
func IsMiss(err error) bool {
	return errors.Is(err, redis.Nil)
}
