package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
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

// Run starts the Kafka consumer loop. Each event triggers two side effects:
//   1. Email notification (create/update only, when UserEmail present)
//   2. Cache invalidation: delete the user's first-page todos cache key
//
// Trace context flows in via the W3C traceparent Kafka header, so the
// consumer span is a child of the producer's span — the entire pipeline
// shows up as one trace in Jaeger.
func Run(ctx context.Context, rdb *redis.Client) {
	topic := envOrDefault("KAFKA_TOPIC", "todos")
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
			slog.Error("kafka read error", "err", err)
			continue
		}
		handleMessage(ctx, rdb, topic, msg)
	}
}

func handleMessage(parent context.Context, rdb *redis.Client, topic string, msg kafka.Message) {
	// Extract upstream trace context from Kafka headers — that's how the
	// trace continues from todo-service across the broker.
	carrier := propagation.MapCarrier{}
	for _, h := range msg.Headers {
		carrier[h.Key] = string(h.Value)
	}
	ctx := otel.GetTextMapPropagator().Extract(parent, carrier)
	ctx, span := tracer.Start(ctx, "kafka.consume "+topic,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.kafka.message.key", string(msg.Key)),
			attribute.Int64("messaging.kafka.partition", int64(msg.Partition)),
			attribute.Int64("messaging.kafka.offset", msg.Offset),
		),
	)
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

	// 1. Email — fire on create/update with a recipient.
	if (event.Action == models.ActionCreate || event.Action == models.ActionUpdate) && event.UserEmail != "" {
		subject, text, html := email.RenderTodo(event.Action, event.Todo)
		email.Send(ctx, event.UserEmail, "", subject, text, html)
	}

	// 2. Cache invalidation — delete the user's todo-list cache regardless of action.
	if rdb != nil && event.Todo.UserID != "" {
		invalidateUserTodoListCache(ctx, rdb, event.Todo.UserID)
	}

	obs.KafkaConsumedTotal.WithLabelValues(topic, "ok").Inc()
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
