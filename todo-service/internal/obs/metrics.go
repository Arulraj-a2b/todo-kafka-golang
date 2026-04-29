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

	DBQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "DB query latency.",
			Buckets: []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
		},
		[]string{"operation"},
	)
	DBQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "db_queries_total", Help: "DB queries by operation and status."},
		[]string{"operation", "status"},
	)
	DBConnectionsInUse = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_in_use", Help: "Open DB connections currently in use.",
	})
	DBConnectionsIdle = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_idle", Help: "Idle DB connections in the pool.",
	})

	CacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "cache_hits_total", Help: "Cache hits by key prefix."},
		[]string{"key_prefix"},
	)
	CacheMissesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "cache_misses_total", Help: "Cache misses by key prefix."},
		[]string{"key_prefix"},
	)

	ExternalAPICallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "external_api_calls_total", Help: "Calls to external APIs by vendor and status."},
		[]string{"vendor", "status"},
	)
	ExternalAPICallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "external_api_call_duration_seconds",
			Help:    "External API call latency.",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"vendor"},
	)

	KafkaPublishedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "kafka_messages_published_total", Help: "Kafka messages published."},
		[]string{"topic", "status"},
	)
	KafkaConsumedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "kafka_messages_consumed_total", Help: "Kafka messages consumed."},
		[]string{"topic", "status"},
	)
)

func InitMetrics() {
	Reg.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
		HTTPRequestsTotal, HTTPRequestDuration, HTTPInFlight,
		DBQueryDuration, DBQueriesTotal, DBConnectionsInUse, DBConnectionsIdle,
		CacheHitsTotal, CacheMissesTotal,
		ExternalAPICallsTotal, ExternalAPICallDuration,
		KafkaPublishedTotal, KafkaConsumedTotal,
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
		userID, _ := c.Get("user_id")
		slog.InfoContext(c.Request.Context(), "request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"user_id", userID,
		)
	}
}

func MountMetrics(r *gin.Engine) {
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(Reg, promhttp.HandlerOpts{Registry: Reg})))
}
