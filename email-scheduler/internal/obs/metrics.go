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
	OverdueScanned = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "scheduler_overdue_scanned_total",
		Help: "Total overdue todos surfaced by the scan (pre-dedup).",
	})
	OverduePublished = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "scheduler_overdue_published_total",
		Help: "Overdue events published to Kafka (post-dedup).",
	})
	OverdueSkipped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "scheduler_overdue_skipped_dedup_total",
		Help: "Overdue todos skipped because already notified.",
	})

	SummaryUsersProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "scheduler_summary_users_processed_total",
		Help: "Users for whom a daily summary was attempted.",
	})
	SummaryPublished = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "scheduler_summary_published_total",
		Help: "Daily summary events published to Kafka.",
	})

	ScanDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "scheduler_scan_duration_seconds",
			Help:    "Wall-clock time of a full scheduler pass.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"job"},
	)

	KafkaPublishedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "kafka_messages_published_total", Help: "Kafka messages published."},
		[]string{"topic", "status"},
	)
	ExternalAPICallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "external_api_call_duration_seconds",
			Help:    "External API call latency.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"vendor"},
	)
)

func InitMetrics() {
	Reg.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
		OverdueScanned, OverduePublished, OverdueSkipped,
		SummaryUsersProcessed, SummaryPublished,
		ScanDuration,
		KafkaPublishedTotal, ExternalAPICallDuration,
	)
}

// StartMetricsServer runs a tiny HTTP server in a goroutine exposing /metrics
// + /healthz on METRICS_PORT (default 9101 — 9100 is taken by notification-worker).
func StartMetricsServer() {
	port := os.Getenv("METRICS_PORT")
	if port == "" {
		port = "9101"
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
