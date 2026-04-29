package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"

	"notification-worker/email"
	"notification-worker/internal/obs"
	"notification-worker/models"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("notification-worker.consumer")

// Run starts two parallel Kafka consumer loops:
//
//   - todos topic       — CRUD events from todo-service handlers. Drives
//                         create/update emails AND cache invalidation.
//   - email.events topic — scheduled events from email-scheduler. Drives
//                          overdue + daily-summary emails. No cache logic.
//
// Each loop runs in its own goroutine. Run blocks until ctx is cancelled.
func Run(ctx context.Context, rdb *redis.Client) {
	todosTopic := envOrDefault("KAFKA_TOPIC", "todos")
	emailTopic := envOrDefault("KAFKA_EMAIL_TOPIC", "email.events")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		runTodosLoop(ctx, rdb, todosTopic)
	}()
	go func() {
		defer wg.Done()
		runEmailLoop(ctx, emailTopic)
	}()
	wg.Wait()
}

// ---------- todos topic loop (CRUD events) ----------

func runTodosLoop(ctx context.Context, rdb *redis.Client, topic string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{os.Getenv("KAFKA_BROKER")},
		Topic:       topic,
		GroupID:     envOrDefault("KAFKA_GROUP_ID", "notification-worker"),
		StartOffset: kafka.LastOffset,
	})
	defer reader.Close()
	slog.Info("notification-worker consuming Kafka", "topic", topic)

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("kafka read error", "topic", topic, "err", err)
			continue
		}
		handleTodosMessage(ctx, rdb, topic, msg)
	}
}

func handleTodosMessage(parent context.Context, rdb *redis.Client, topic string, msg kafka.Message) {
	ctx, span := startConsumerSpan(parent, topic, msg)
	defer span.End()

	start := time.Now()
	defer func() {
		obs.KafkaConsumeDuration.WithLabelValues(topic).Observe(time.Since(start).Seconds())
	}()

	var event models.TodoEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		obs.KafkaConsumedTotal.WithLabelValues(topic, "decode_error").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, "decode failed")
		slog.ErrorContext(ctx, "kafka decode error", "err", err, "raw", string(msg.Value))
		return
	}

	span.SetAttributes(
		attribute.String("todo.id", event.Todo.ID),
		attribute.String("todo.user_id", event.Todo.UserID),
		attribute.String("todo.action", event.Action),
	)

	// 1. Email on create/update.
	if (event.Action == models.ActionCreate || event.Action == models.ActionUpdate) && event.UserEmail != "" {
		subject, text, html := email.RenderTodo(event.Action, event.Todo)
		email.Send(ctx, event.UserEmail, "", subject, text, html)
	}

	// 2. Cache invalidation regardless of action.
	if rdb != nil && event.Todo.UserID != "" {
		invalidateUserTodoListCache(ctx, rdb, event.Todo.UserID)
	}

	obs.KafkaConsumedTotal.WithLabelValues(topic, "ok").Inc()
}

// ---------- email.events topic loop (scheduled events) ----------

func runEmailLoop(ctx context.Context, topic string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{os.Getenv("KAFKA_BROKER")},
		Topic:       topic,
		GroupID:     envOrDefault("KAFKA_EMAIL_GROUP_ID", "notification-worker-email"),
		StartOffset: kafka.LastOffset,
	})
	defer reader.Close()
	slog.Info("notification-worker consuming Kafka", "topic", topic)

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("kafka read error", "topic", topic, "err", err)
			continue
		}
		handleEmailMessage(ctx, topic, msg)
	}
}

func handleEmailMessage(parent context.Context, topic string, msg kafka.Message) {
	ctx, span := startConsumerSpan(parent, topic, msg)
	defer span.End()

	start := time.Now()
	defer func() {
		obs.KafkaConsumeDuration.WithLabelValues(topic).Observe(time.Since(start).Seconds())
	}()

	var envelope models.EmailEvent
	if err := json.Unmarshal(msg.Value, &envelope); err != nil {
		obs.KafkaConsumedTotal.WithLabelValues(topic, "decode_error").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, "envelope decode failed")
		slog.ErrorContext(ctx, "envelope decode error", "err", err, "raw", string(msg.Value))
		return
	}
	span.SetAttributes(attribute.String("email.event_type", envelope.Type))

	switch envelope.Type {
	case models.EmailTypeOverdue:
		var data models.OverdueData
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			obs.KafkaConsumedTotal.WithLabelValues(topic, "decode_error").Inc()
			slog.ErrorContext(ctx, "overdue payload decode failed", "err", err)
			return
		}
		if data.UserEmail == "" {
			slog.WarnContext(ctx, "overdue event missing user email; skipping", "todo_id", data.Todo.ID)
			obs.KafkaConsumedTotal.WithLabelValues(topic, "skipped").Inc()
			return
		}
		subject, text, html := email.RenderOverdue(data)
		email.Send(ctx, data.UserEmail, "", subject, text, html)

	case models.EmailTypeDailySummary:
		var data models.DailySummaryData
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			obs.KafkaConsumedTotal.WithLabelValues(topic, "decode_error").Inc()
			slog.ErrorContext(ctx, "summary payload decode failed", "err", err)
			return
		}
		if data.UserEmail == "" {
			slog.WarnContext(ctx, "summary event missing user email; skipping", "user_id", data.UserID)
			obs.KafkaConsumedTotal.WithLabelValues(topic, "skipped").Inc()
			return
		}
		subject, text, html := email.RenderDailySummary(data)
		email.Send(ctx, data.UserEmail, "", subject, text, html)

	default:
		obs.KafkaConsumedTotal.WithLabelValues(topic, "unknown_type").Inc()
		slog.WarnContext(ctx, "unknown email event type; ignoring", "type", envelope.Type)
		return
	}
	obs.KafkaConsumedTotal.WithLabelValues(topic, "ok").Inc()
}

// ---------- helpers ----------

// startConsumerSpan extracts the W3C trace context from Kafka headers and
// starts a child span. That's what stitches scheduler/todo-service producer
// spans to the consumer side in Jaeger.
func startConsumerSpan(parent context.Context, topic string, msg kafka.Message) (context.Context, trace.Span) {
	carrier := propagation.MapCarrier{}
	for _, h := range msg.Headers {
		carrier[h.Key] = string(h.Value)
	}
	ctx := otel.GetTextMapPropagator().Extract(parent, carrier)
	return tracer.Start(ctx, "kafka.consume "+topic,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.kafka.message.key", string(msg.Key)),
			attribute.Int64("messaging.kafka.partition", int64(msg.Partition)),
			attribute.Int64("messaging.kafka.offset", msg.Offset),
		),
	)
}

// invalidateUserTodoListCache deletes all cached first-page entries for a user.
// Pattern: todos:list:{userID}:* (covers different status filters).
func invalidateUserTodoListCache(ctx context.Context, rdb *redis.Client, userID string) {
	pattern := "todos:list:" + userID + ":*"
	iter := rdb.Scan(ctx, 0, pattern, 100).Iterator()
	keys := []string{}
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		slog.WarnContext(ctx, "cache invalidate scan failed", "pattern", pattern, "err", err)
		return
	}
	if len(keys) == 0 {
		return
	}
	if err := rdb.Del(ctx, keys...).Err(); err != nil {
		slog.WarnContext(ctx, "cache invalidate del failed", "keys", keys, "err", err)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
