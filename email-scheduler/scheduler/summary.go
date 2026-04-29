package scheduler

import (
	"context"
	"log/slog"
	"time"

	"email-scheduler/internal/obs"
	"email-scheduler/models"
	"email-scheduler/publisher"
	"email-scheduler/store"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	summaryBatchSize       = 500
	summaryHighlightLimit  = 5
	summaryInterBatchSleep = 200 * time.Millisecond // throttle so 1M users span ~the morning, not all at once
)

// RunDailySummary scans every active user and publishes a "user.daily_summary"
// event. Called by the cron at 9 AM (or whatever DAILY_SUMMARY_CRON is set to).
// Idempotent via Redis dedup keyed on (date, userID) — safe to fire multiple
// times the same day.
func RunDailySummary(ctx context.Context, db *store.DB, dedup *store.Dedup, pub *publisher.Publisher) {
	ctx, span := tracer.Start(ctx, "scheduler.run_daily_summary", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	start := time.Now()
	defer func() {
		obs.ScanDuration.WithLabelValues("daily_summary").Observe(time.Since(start).Seconds())
	}()

	date := time.Now().UTC().Format("2006-01-02")
	slog.InfoContext(ctx, "daily summary started", "date", date)

	cursor := ""
	totalUsers := 0
	totalPublished := 0

	for {
		batch, err := db.UserIDsWithActiveTodos(ctx, cursor, summaryBatchSize)
		if err != nil {
			slog.ErrorContext(ctx, "summary user batch failed", "err", err, "after", cursor)
			return
		}
		if len(batch) == 0 {
			break
		}

		published := processSummaryBatch(ctx, db, dedup, pub, date, batch)
		totalUsers += len(batch)
		totalPublished += published

		cursor = batch[len(batch)-1]

		// Cooperative yield + throttle so we don't hammer SendGrid.
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "daily summary cancelled", "users_so_far", totalUsers)
			return
		case <-time.After(summaryInterBatchSleep):
		}
	}

	span.SetAttributes(
		attribute.Int("summary.users", totalUsers),
		attribute.Int("summary.published", totalPublished),
	)
	slog.InfoContext(ctx, "daily summary complete",
		"date", date, "users", totalUsers, "published", totalPublished,
		"duration_ms", time.Since(start).Milliseconds())
}

// processSummaryBatch handles one page of users: aggregate counts in one query,
// look up emails in one query, then publish one Kafka event per user.
func processSummaryBatch(
	ctx context.Context,
	db *store.DB, dedup *store.Dedup, pub *publisher.Publisher,
	date string, userIDs []string,
) int {
	counts, err := db.SummaryCountsForUsers(ctx, userIDs)
	if err != nil {
		slog.ErrorContext(ctx, "summary counts failed", "err", err)
		return 0
	}
	emails, err := db.LookupUserEmails(ctx, userIDs)
	if err != nil {
		slog.ErrorContext(ctx, "summary email lookup failed", "err", err)
		return 0
	}

	published := 0
	for _, uid := range userIDs {
		obs.SummaryUsersProcessed.Inc()
		c, ok := counts[uid]
		if !ok {
			// User had a deleted/completed-yesterday todo but no active ones any more.
			continue
		}
		email := emails[uid]
		if email == "" {
			continue
		}
		// Peek at dedup BEFORE doing work, but don't mark yet — that way a
		// transient Kafka failure doesn't leave the user unreachable for
		// the rest of the day.
		if dedup.IsSummaryAlreadySent(ctx, date, uid) {
			continue
		}

		highlights, _ := db.HighlightsForUser(ctx, uid, summaryHighlightLimit)

		data := models.DailySummaryData{
			UserID:    uid,
			UserEmail: email,
			Date:      date,
			Counts: models.DailySummaryCounts{
				Pending:            c.Pending,
				InProgress:         c.InProgress,
				DueToday:           c.DueToday,
				Overdue:            c.Overdue,
				CompletedYesterday: c.CompletedYesterday,
			},
			Highlights: highlights,
		}
		if err := pub.PublishDailySummary(ctx, data); err != nil {
			slog.WarnContext(ctx, "summary publish failed", "user_id", uid, "err", err)
			continue
		}
		// Mark only AFTER the publish succeeded. Worst-case duplicate is a
		// scheduler crash between publish and mark — small and acceptable.
		dedup.MarkSummaryIfNew(ctx, date, uid)
		obs.SummaryPublished.Inc()
		published++
	}
	return published
}
