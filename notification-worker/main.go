package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"notification-worker/consumer"
	"notification-worker/internal/obs"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	_ = godotenv.Load()

	obs.InitLogger("notification-worker")
	obs.InitMetrics()
	obs.StartMetricsServer()

	rootCtx := context.Background()
	shutdown := obs.InitTracer(rootCtx, "notification-worker")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			slog.Error("tracer shutdown", "err", err)
		}
	}()

	if os.Getenv("KAFKA_BROKER") == "" {
		slog.Error("KAFKA_BROKER not set")
		os.Exit(1)
	}

	rdb := newRedis()
	if rdb != nil {
		defer rdb.Close()
	}

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		slog.Info("shutting down")
		cancel()
	}()

	consumer.Run(ctx, rdb)
}

func newRedis() *redis.Client {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if url == "" {
		slog.Info("REDIS_URL not set; cache invalidation disabled")
		return nil
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		slog.Warn("invalid REDIS_URL; cache invalidation disabled", "url", url, "err", err)
		return nil
	}
	c := redis.NewClient(opt)
	if err := c.Ping(context.Background()).Err(); err != nil {
		slog.Warn("redis unreachable; cache invalidation disabled", "url", url, "err", err)
		return nil
	}
	slog.Info("redis connected (cache invalidator active)", "url", url)
	return c
}
