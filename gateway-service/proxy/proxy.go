package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// New returns a Gin handler that reverse-proxies to target, stripping
// stripPrefix from the inbound path before forwarding. The proxy's HTTP
// transport is wrapped with otelhttp so traceparent headers propagate to the
// upstream service — that's what stitches the gateway and downstream service
// spans into one continuous trace in Jaeger.
func New(target, stripPrefix string) gin.HandlerFunc {
	u, err := url.Parse(target)
	if err != nil {
		slog.Error("invalid proxy target", "target", target, "err", err)
		panic(err)
	}

	rp := httputil.NewSingleHostReverseProxy(u)
	rp.Transport = otelhttp.NewTransport(http.DefaultTransport,
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return r.Method + " " + target + r.URL.Path
		}),
	)

	defaultDirector := rp.Director
	rp.Director = func(req *http.Request) {
		defaultDirector(req)
		if stripPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, stripPrefix)
			if !strings.HasPrefix(req.URL.Path, "/") {
				req.URL.Path = "/" + req.URL.Path
			}
		}
		req.Host = u.Host
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.WarnContext(r.Context(), "proxy upstream error",
			"method", r.Method, "path", r.URL.Path, "target", target, "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream_unavailable"}`))
	}

	return func(c *gin.Context) {
		// Pass the request context (now carrying the otelgin span) into the
		// proxy so otelhttp.Transport can read it and inject traceparent.
		rp.ServeHTTP(c.Writer, c.Request.WithContext(c.Request.Context()))
	}
}
