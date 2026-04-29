package obs

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// InitLogger sets the default slog handler. Format and level come from env:
//   LOG_FORMAT=json|text   (default text in dev, json in compose)
//   LOG_LEVEL=debug|info|warn|error  (default info)
//
// All log records get a constant `service` attribute. Records made via
// slog.*Context with an OTel span in context get `trace_id` and `span_id`
// fields automatically — that is the bridge from logs to Jaeger.
func InitLogger(serviceName string) {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	opts := &slog.HandlerOptions{Level: level}

	var base slog.Handler
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "json" {
		base = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		base = slog.NewTextHandler(os.Stdout, opts)
	}

	withService := base.WithAttrs([]slog.Attr{slog.String("service", serviceName)})
	slog.SetDefault(slog.New(&traceHandler{Handler: withService}))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// traceHandler decorates the inner handler with OTel trace fields when the
// caller passed a context (slog.InfoContext etc.). Plain slog.Info loses the
// correlation — that's intentional for non-request paths like main.go.
type traceHandler struct {
	slog.Handler
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		sc := span.SpanContext()
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithGroup(name)}
}
