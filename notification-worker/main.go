package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"notification-worker/consumer"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	if os.Getenv("KAFKA_BROKER") == "" {
		log.Fatal("KAFKA_BROKER not set")
	}

	rdb := newRedis()
	if rdb != nil {
		defer rdb.Close()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		log.Println("shutting down...")
		cancel()
	}()

	consumer.Run(ctx, rdb)
}

func newRedis() *redis.Client {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if url == "" {
		log.Println("REDIS_URL not set; cache invalidation disabled")
		return nil
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("invalid REDIS_URL %q: %v; cache invalidation disabled", url, err)
		return nil
	}
	c := redis.NewClient(opt)
	if err := c.Ping(context.Background()).Err(); err != nil {
		log.Printf("redis unreachable at %s: %v; cache invalidation disabled", url, err)
		return nil
	}
	log.Printf("Redis connected at %s (cache invalidator active)", url)
	return c
}
