package store

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// Dedup wraps a Redis client with the two notification-tracking keys we use:
//   email:overdue:sent:{todoID}       TTL 7d
//   email:summary:sent:{date}:{user}  TTL 25h
//
// Methods tolerate a nil receiver — if Redis is unavailable, we'd rather
// occasionally double-send than block the scheduler.
type Dedup struct {
	rdb *redis.Client
}

const (
	overdueTTL = 7 * 24 * time.Hour
	summaryTTL = 25 * time.Hour
)

func NewDedup() *Dedup {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if url == "" {
		slog.Info("REDIS_URL not set; dedup disabled (will resend on every scan)")
		return &Dedup{}
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		slog.Warn("invalid REDIS_URL; dedup disabled", "url", url, "err", err)
		return &Dedup{}
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Warn("redis unreachable; dedup disabled", "url", url, "err", err)
		return &Dedup{}
	}
	if err := redisotel.InstrumentTracing(rdb); err != nil {
		slog.Warn("redisotel tracing", "err", err)
	}
	slog.Info("dedup connected", "url", url)
	return &Dedup{rdb: rdb}
}

func (d *Dedup) Close() {
	if d != nil && d.rdb != nil {
		_ = d.rdb.Close()
	}
}

// MarkOverdueIfNew returns true if the todo had not been notified before
// (and is now marked). Returns false if it was already marked. Atomic via SETNX.
// On Redis error or disabled state, returns true (fail-open: better to send than skip).
func (d *Dedup) MarkOverdueIfNew(ctx context.Context, todoID string) bool {
	if d == nil || d.rdb == nil {
		return true
	}
	ok, err := d.rdb.SetNX(ctx, overdueKey(todoID), "1", overdueTTL).Result()
	if err != nil {
		slog.WarnContext(ctx, "dedup setnx failed; sending anyway", "todo_id", todoID, "err", err)
		return true
	}
	return ok
}

// MarkSummaryIfNew is the same idea for daily summaries, keyed by (date, userID).
func (d *Dedup) MarkSummaryIfNew(ctx context.Context, date, userID string) bool {
	if d == nil || d.rdb == nil {
		return true
	}
	ok, err := d.rdb.SetNX(ctx, summaryKey(date, userID), "1", summaryTTL).Result()
	if err != nil {
		slog.WarnContext(ctx, "dedup setnx failed; sending anyway", "user_id", userID, "err", err)
		return true
	}
	return ok
}

// IsOverdueAlreadySent is a peek that does NOT mark — useful for the metrics
// distinguishing "scanned" from "skipped".
func (d *Dedup) IsOverdueAlreadySent(ctx context.Context, todoID string) bool {
	return d.exists(ctx, overdueKey(todoID))
}

// IsSummaryAlreadySent is the same peek for daily summaries.
func (d *Dedup) IsSummaryAlreadySent(ctx context.Context, date, userID string) bool {
	return d.exists(ctx, summaryKey(date, userID))
}

func (d *Dedup) exists(ctx context.Context, key string) bool {
	if d == nil || d.rdb == nil {
		return false
	}
	_, err := d.rdb.Get(ctx, key).Result()
	if err == nil {
		return true
	}
	if errors.Is(err, redis.Nil) {
		return false
	}
	slog.WarnContext(ctx, "dedup get failed", "key", key, "err", err)
	return false
}

func overdueKey(todoID string) string         { return "email:overdue:sent:" + todoID }
func summaryKey(date, userID string) string   { return "email:summary:sent:" + date + ":" + userID }
