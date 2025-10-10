package utils

import (
	"time"

	"github.com/IBM/pgxpoolprometheus"
	"github.com/checkmarble/marble-backend/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MetricRequestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Number of HTTP requests",
	}, []string{"org_id", "method", "url", "status"})

	MetricRequestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_requests_latency",
		Help:    "Latency of HTTP requests",
		Buckets: []float64{0.2, 0.3, 0.6, 1.0, 2.0},
	}, []string{"org_id", "method", "url", "status"})

	MetricDecisionLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "marble_decision_latency",
		Help:    "Latency of decisions",
		Buckets: []float64{0.1, 0.2, 0.3, 0.6, 1.0},
	}, []string{"org_id"})

	MetricScreeningLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "marble_screening_latency",
		Help:    "Latency of screenings",
		Buckets: []float64{0.1, 0.2, 0.3, 0.6, 1.0},
	}, []string{"org_id"})

	MetricIngestionCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marble_ingestion_total",
		Help: "Number of objects ingested",
	}, []string{"org_id"})

	MetricIngestionLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "marble_ingestion_latency",
		Help: "Latency of object ingestion",
	}, []string{"org_id"})

	MetricQueryLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "sql_query_latency",
		Help: "Latency of SQL queries",
	}, []string{"org_id"})

	MetricJobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "job_duration",
		Help: "Duration of asynchronous worker jobs",
	}, []string{"queue", "job_name"})
)

func InitPgxPrometheus(pool *pgxpool.Pool, org *models.Organization) {
	labels := map[string]string{
		"db_type": "main",
	}

	if org != nil {
		labels["db_type"] = "client_data"
		labels["org_id"] = org.Id
	}

	prometheus.MustRegister(pgxpoolprometheus.NewCollector(pool, labels))
}

func MeasureLatencyErr[R any](metric *prometheus.HistogramVec, labels prometheus.Labels, f func() (R, error)) (R, error) {
	start := time.Now()
	result, err := f()

	metric.With(labels).Observe(time.Since(start).Seconds())

	return result, err
}

func MeasureLatency[R any](metric *prometheus.HistogramVec, labels prometheus.Labels, f func() R) R {
	start := time.Now()
	result := f()

	metric.With(labels).Observe(time.Since(start).Seconds())

	return result
}
