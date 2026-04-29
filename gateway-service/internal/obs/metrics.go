package obs

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var Reg = prometheus.NewRegistry()

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "HTTP requests by method, path, status."},
		[]string{"method", "path", "status"},
	)
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)
	HTTPInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight", Help: "In-flight HTTP requests.",
	})

	// Gateway-specific: rate-limit decisions per route group.
	RateLimitDecisions = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "rate_limit_decisions_total", Help: "Rate-limit decisions by bucket and decision."},
		[]string{"bucket", "decision"},
	)
)

func InitMetrics() {
	Reg.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
		HTTPRequestsTotal, HTTPRequestDuration, HTTPInFlight,
		RateLimitDecisions,
	)
}

func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		HTTPInFlight.Inc()
		defer HTTPInFlight.Dec()
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
		HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
	}
}

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		slog.InfoContext(c.Request.Context(), "request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

func MountMetrics(r *gin.Engine) {
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(Reg, promhttp.HandlerOpts{Registry: Reg})))
}
