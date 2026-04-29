package obs

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTracer wires the global OTel TracerProvider against a Jaeger OTLP/gRPC
// endpoint. Returns a shutdown func — call from main with a defer.
//
// Env:
//   OTEL_EXPORTER_OTLP_ENDPOINT (default jaeger:4317)
//   OTEL_SERVICE_NAME (overrides serviceName arg)
//   TRACE_SAMPLE_RATE (0.0–1.0; default 1.0 in dev, 0.1 recommended in prod)
func InitTracer(ctx context.Context, serviceName string) func(context.Context) error {
	if v := os.Getenv("OTEL_SERVICE_NAME"); v != "" {
		serviceName = v
	}
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "jaeger:4317"
	}
	sampleRate := 1.0
	if v := os.Getenv("TRACE_SAMPLE_RATE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			sampleRate = f
		}
	}

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	exp, err := otlptracegrpc.New(dialCtx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		slog.Error("otel exporter init failed; tracing disabled", "err", err, "endpoint", endpoint)
		return func(context.Context) error { return nil }
	}

	res, _ := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	))

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRate))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	slog.Info("tracing initialized", "endpoint", endpoint, "service", serviceName, "sample_rate", sampleRate)
	return tp.Shutdown
}
