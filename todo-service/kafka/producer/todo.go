package producer

import (
	"context"
	"log/slog"
	"os"
	"time"

	"todo-service/internal/obs"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var KafkaWriter *kafka.Writer

var tracer = otel.Tracer("kafka.producer")

func InitKafka() {
	KafkaWriter = &kafka.Writer{
		Addr:         kafka.TCP(os.Getenv("KAFKA_BROKER")),
		Topic:        os.Getenv("KAFKA_TOPIC"),
		BatchTimeout: 10 * time.Millisecond,
	}
}

// Publish writes a message and:
//   1. Creates an OTel span "kafka.publish topic=…" so the producer leg shows
//      up in Jaeger.
//   2. Injects the W3C traceparent into Kafka message headers so the consumer
//      can continue the same trace (cross-process propagation).
//   3. Increments kafka_messages_published_total counter.
//
// Best-effort: logs and returns nil on error, since the DB write has already
// committed. Set the span status to error so it shows up in Jaeger.
func Publish(ctx context.Context, key, value []byte) {
	if KafkaWriter == nil {
		return
	}
	topic := KafkaWriter.Topic
	ctx, span := tracer.Start(ctx, "kafka.publish "+topic,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.kafka.message.key", string(key)),
		),
	)
	defer span.End()

	headers := injectHeaders(ctx)

	start := time.Now()
	err := KafkaWriter.WriteMessages(ctx, kafka.Message{Key: key, Value: value, Headers: headers})
	dur := time.Since(start).Seconds()
	obs.ExternalAPICallDuration.WithLabelValues("kafka").Observe(dur)
	if err != nil {
		obs.KafkaPublishedTotal.WithLabelValues(topic, "error").Inc()
		obs.ExternalAPICallsTotal.WithLabelValues("kafka", "error").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "kafka publish failed", "err", err, "topic", topic, "key", string(key))
		return
	}
	obs.KafkaPublishedTotal.WithLabelValues(topic, "ok").Inc()
	obs.ExternalAPICallsTotal.WithLabelValues("kafka", "ok").Inc()
}

// injectHeaders puts the active span context into Kafka message headers using
// the W3C TraceContext propagator (default OTel propagator).
func injectHeaders(ctx context.Context) []kafka.Header {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	headers := make([]kafka.Header, 0, len(carrier))
	for k, v := range carrier {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}
	return headers
}
