package utils

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MetricRequestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marble_http_requests_total",
		Help: "Number of HTTP requests",
	}, []string{"org_id", "method", "url", "status"})

	MetricRequestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "marble_http_requests_latency",
		Help:    "Latency of HTTP requests",
		Buckets: []float64{0.2, 0.3, 0.6, 1.0, 2.0},
	}, []string{"org_id", "method", "url", "status"})

	MetricDecisionCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marble_decision_total",
		Help: "Latency of decisions",
	}, []string{"org_id"})

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
		Name: "marble_sql_query_latency",
		Help: "Latency of SQL queries",
	}, []string{"org_id", "schema"})

	MetricJobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "marble_job_duration",
		Help: "Duration of asynchronous worker jobs",
	}, []string{"queue", "job_name"})
)

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
