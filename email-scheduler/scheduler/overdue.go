package scheduler

import (
	"context"
	"log/slog"
	"time"

	"email-scheduler/internal/obs"
	"email-scheduler/models"
	"email-scheduler/publisher"
	"email-scheduler/store"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const overdueScanLimit = 1000

var tracer = otel.Tracer("email-scheduler.scheduler")

// RunOverdue starts a ticker that scans for overdue todos and publishes
// "todo.overdue" events. Blocks until ctx is cancelled.
func RunOverdue(ctx context.Context, db *store.DB, dedup *store.Dedup, pub *publisher.Publisher, interval time.Duration) {
	slog.Info("overdue scanner started", "interval", interval)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("overdue scanner stopping")
			return
		case <-t.C:
			scanOverdueOnce(ctx, db, dedup, pub)
		}
	}
}

func scanOverdueOnce(parent context.Context, db *store.DB, dedup *store.Dedup, pub *publisher.Publisher) {
	ctx, span := tracer.Start(parent, "scheduler.scan_overdue", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	start := time.Now()
	defer func() {
		obs.ScanDuration.WithLabelValues("overdue").Observe(time.Since(start).Seconds())
	}()

	todos, err := db.FetchOverdueTodos(ctx, overdueScanLimit)
	if err != nil {
		slog.ErrorContext(ctx, "overdue fetch failed", "err", err)
		return
	}
	span.SetAttributes(attribute.Int("overdue.count", len(todos)))
	obs.OverdueScanned.Add(float64(len(todos)))

	if len(todos) == 0 {
		return
	}

	// Look up emails in one batch from auth_db.
	userIDs := make([]string, 0, len(todos))
	seen := map[string]struct{}{}
	for _, t := range todos {
		if _, ok := seen[t.UserID]; ok {
			continue
		}
		seen[t.UserID] = struct{}{}
		userIDs = append(userIDs, t.UserID)
	}
	emails, err := db.LookupUserEmails(ctx, userIDs)
	if err != nil {
		slog.ErrorContext(ctx, "email lookup failed", "err", err)
		return
	}

	now := time.Now()
	published := 0
	for _, t := range todos {
		email := emails[t.UserID]
		if email == "" {
			continue
		}
		// Peek before doing work; don't mark dedup until publish succeeds.
		// A failed publish leaves the dedup unset so the next scan retries.
		if dedup.IsOverdueAlreadySent(ctx, t.ID) {
			obs.OverdueSkipped.Inc()
			continue
		}
		due := time.Time{}
		if t.DueDate != nil {
			due = *t.DueDate
		}
		data := models.OverdueData{
			Todo: models.Todo{
				ID: t.ID, UserID: t.UserID, Title: t.Title, Status: t.Status,
				Priority: t.Priority, DueDate: t.DueDate, Tags: t.Tags,
				CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
			},
			UserEmail:        email,
			OverdueBySeconds: int64(now.Sub(due).Seconds()),
		}
		if err := pub.PublishOverdue(ctx, data); err != nil {
			slog.WarnContext(ctx, "overdue publish failed", "todo_id", t.ID, "err", err)
			continue
		}
		// Mark dedup only after the publish actually succeeded.
		dedup.MarkOverdueIfNew(ctx, t.ID)
		obs.OverduePublished.Inc()
		published++
	}
	slog.InfoContext(ctx, "overdue scan complete",
		"scanned", len(todos), "published", published, "duration_ms", time.Since(start).Milliseconds())
}
