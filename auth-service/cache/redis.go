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

// Client wraps redis.Client with fail-open semantics. Methods on a nil/disabled
// client are no-ops so callers can fall back to the database.
type Client struct {
	rdb *redis.Client
}

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

func IsMiss(err error) bool {
	return errors.Is(err, redis.Nil)
}
