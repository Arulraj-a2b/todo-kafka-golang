package obs

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var Reg = prometheus.NewRegistry()

var (
	KafkaConsumedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "kafka_messages_consumed_total", Help: "Kafka messages consumed."},
		[]string{"topic", "status"},
	)
	KafkaConsumeDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kafka_consume_duration_seconds",
			Help:    "Time to handle a single Kafka message end-to-end.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"topic"},
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
)

func InitMetrics() {
	Reg.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
		KafkaConsumedTotal, KafkaConsumeDuration,
		ExternalAPICallsTotal, ExternalAPICallDuration,
	)
}

// StartMetricsServer runs a tiny HTTP server in the background exposing
// /metrics and /healthz on METRICS_PORT (default 9100). Notification-worker
// has no other HTTP surface so it gets a dedicated server.
func StartMetricsServer() {
	port := os.Getenv("METRICS_PORT")
	if port == "" {
		port = "9100"
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(Reg, promhttp.HandlerOpts{Registry: Reg}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	srv := &http.Server{Addr: "0.0.0.0:" + port, Handler: mux}
	go func() {
		slog.Info("metrics server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics server exit", "err", err)
		}
	}()
}
