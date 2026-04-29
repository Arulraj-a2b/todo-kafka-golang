package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"email-scheduler/internal/obs"
	"email-scheduler/publisher"
	"email-scheduler/scheduler"
	"email-scheduler/store"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
)

func main() {
	_ = godotenv.Load()

	obs.InitLogger("email-scheduler")
	obs.InitMetrics()
	obs.StartMetricsServer()

	rootCtx := context.Background()
	shutdownTracer := obs.InitTracer(rootCtx, "email-scheduler")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracer(ctx)
	}()

	db := store.InitDB()
	defer db.Close()

	dedup := store.NewDedup()
	defer dedup.Close()

	pub := publisher.New()
	defer pub.Close()

	overdueInterval := parseDuration("OVERDUE_SCAN_INTERVAL", 5*time.Minute)
	summarySpec := envOrDefault("DAILY_SUMMARY_CRON", "0 9 * * *")

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Overdue scanner — runs in its own goroutine on a fixed-interval ticker.
	go scheduler.RunOverdue(ctx, db, dedup, pub, overdueInterval)

	// Daily summary — driven by cron. robfig/cron handles parsing and timing.
	c := cron.New(cron.WithLocation(time.UTC))
	if _, err := c.AddFunc(summarySpec, func() {
		// Each fire gets its own root context so a long run isn't cancelled
		// when ctx is cancelled mid-summary (let it drain naturally).
		scheduler.RunDailySummary(ctx, db, dedup, pub)
	}); err != nil {
		slog.Error("invalid DAILY_SUMMARY_CRON", "spec", summarySpec, "err", err)
		os.Exit(1)
	}
	c.Start()
	defer c.Stop()
	slog.Info("scheduler started",
		"overdue_interval", overdueInterval,
		"daily_summary_cron", summarySpec,
		"next_summary_run", c.Entries()[0].Next.Format(time.RFC3339))

	<-stop
	slog.Info("shutting down")
	cancel()
	// Give in-flight scans up to 10s to drain.
	time.Sleep(2 * time.Second)
}

func parseDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("invalid duration env; using default", "key", key, "value", v, "default", def)
		return def
	}
	return d
}

func envOrDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
