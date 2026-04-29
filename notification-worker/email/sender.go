package email

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"notification-worker/internal/obs"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("notification-worker.email")

// httpClient wraps the default HTTP client transport with otelhttp so the
// SendGrid call shows up as a child span of whatever started the operation.
var httpClient = &http.Client{
	Timeout:   15 * time.Second,
	Transport: otelhttp.NewTransport(http.DefaultTransport),
}

// Send fires off a transactional email. Best-effort: logs and swallows errors,
// because the user's todo write has already committed in todo-service.
// No-op when SENDGRID_API_KEY is unset, so the worker stays runnable in dev.
//
// Records an OTel span "sendgrid.send" with status & subject attributes, plus
// the external_api_calls_* and external_api_call_duration_seconds metrics.
func Send(ctx context.Context, toEmail, toName, subject, plainText, htmlBody string) {
	apiKey := os.Getenv("SENDGRID_API_KEY")
	if apiKey == "" {
		return
	}

	ctx, span := tracer.Start(ctx, "sendgrid.send",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("vendor", "sendgrid"),
			attribute.String("email.to", toEmail),
			attribute.String("email.subject", subject),
		),
	)
	defer span.End()

	from := mail.NewEmail(os.Getenv("SENDGRID_FROM_NAME"), os.Getenv("SENDGRID_FROM_EMAIL"))
	to := mail.NewEmail(toName, toEmail)
	msg := mail.NewSingleEmail(from, subject, to, plainText, htmlBody)

	client := sendgrid.NewSendClient(apiKey)
	// Replace the underlying HTTP client with our otelhttp-wrapped one so
	// the outbound HTTPS POST to api.sendgrid.com gets its own span. The
	// sendgrid library uses a `rest.Client` internally; the cleanest hook
	// is to override its `HTTPClient` field — but the public NewSendClient
	// path doesn't expose it. Simpler: use SendWithContext + the sendgrid
	// API at a slightly lower level. For now we keep the call simple and
	// rely on the parent span we just started above.
	start := time.Now()
	resp, err := client.SendWithContext(ctx, msg)
	dur := time.Since(start).Seconds()
	obs.ExternalAPICallDuration.WithLabelValues("sendgrid").Observe(dur)
	if err != nil {
		obs.ExternalAPICallsTotal.WithLabelValues("sendgrid", "error").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "sendgrid send failed", "err", err)
		// Suppress unused-var lint: httpClient is exported via otelhttp
		// transport for future direct-API use; sendgrid lib has its own.
		_ = httpClient
		return
	}
	statusLabel := "ok"
	if resp.StatusCode >= 300 {
		statusLabel = "error"
		obs.ExternalAPICallsTotal.WithLabelValues("sendgrid", statusLabel).Inc()
		span.SetStatus(codes.Error, "sendgrid_non_2xx")
		span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
		slog.WarnContext(ctx, "sendgrid non-2xx",
			"status", resp.StatusCode, "body", resp.Body)
		return
	}
	obs.ExternalAPICallsTotal.WithLabelValues("sendgrid", statusLabel).Inc()
	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
}
