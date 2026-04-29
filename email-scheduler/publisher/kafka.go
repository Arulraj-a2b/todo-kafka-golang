package publisher

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"email-scheduler/internal/obs"
	"email-scheduler/models"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("email-scheduler.publisher")

// Publisher is a thin wrapper around kafka.Writer for the email.events topic.
// Mirrors the trace-injection pattern in todo-service so Jaeger can stitch
// scheduler → notification-worker spans into one trace.
type Publisher struct {
	w     *kafka.Writer
	topic string
}

func New() *Publisher {
	topic := envOrDefault("KAFKA_EMAIL_TOPIC", "email.events")
	w := &kafka.Writer{
		Addr:         kafka.TCP(os.Getenv("KAFKA_BROKER")),
		Topic:        topic,
		BatchTimeout: 10 * time.Millisecond,
	}
	slog.Info("publisher ready", "topic", topic, "broker", os.Getenv("KAFKA_BROKER"))
	return &Publisher{w: w, topic: topic}
}

func (p *Publisher) Close() {
	if p != nil && p.w != nil {
		_ = p.w.Close()
	}
}

// PublishOverdue marshals data into an EmailEvent envelope and publishes to
// the email.events topic. Best-effort: logs and returns on error since the
// scheduler's next pass will retry (Redis dedup hasn't been set yet on failure).
func (p *Publisher) PublishOverdue(ctx context.Context, data models.OverdueData) error {
	return p.publish(ctx, models.TypeOverdue, data, []byte(data.Todo.ID))
}

func (p *Publisher) PublishDailySummary(ctx context.Context, data models.DailySummaryData) error {
	return p.publish(ctx, models.TypeDailySummary, data, []byte(data.UserID))
}

func (p *Publisher) publish(ctx context.Context, eventType string, payload any, key []byte) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	envelope, err := json.Marshal(models.EmailEvent{Type: eventType, Data: body})
	if err != nil {
		return err
	}

	ctx, span := tracer.Start(ctx, "kafka.publish "+p.topic,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", p.topic),
			attribute.String("email.event_type", eventType),
		),
	)
	defer span.End()

	headers := injectHeaders(ctx)

	start := time.Now()
	err = p.w.WriteMessages(ctx, kafka.Message{Key: key, Value: envelope, Headers: headers})
	dur := time.Since(start).Seconds()
	obs.ExternalAPICallDuration.WithLabelValues("kafka").Observe(dur)
	if err != nil {
		obs.KafkaPublishedTotal.WithLabelValues(p.topic, "error").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "kafka publish failed", "err", err, "topic", p.topic, "event_type", eventType)
		return err
	}
	obs.KafkaPublishedTotal.WithLabelValues(p.topic, "ok").Inc()
	return nil
}

func injectHeaders(ctx context.Context) []kafka.Header {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	out := make([]kafka.Header, 0, len(carrier))
	for k, v := range carrier {
		out = append(out, kafka.Header{Key: k, Value: []byte(v)})
	}
	return out
}

func envOrDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
