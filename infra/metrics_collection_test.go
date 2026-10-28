package infra

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/utils"
)

func metricsTestContext() context.Context {
	return utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
}

func TestMetricCollectionConfig_ValidateAndFix(t *testing.T) {
	tests := []struct {
		name     string
		in       MetricCollectionConfig
		expected MetricCollectionConfig
	}{
		{
			name: "defaults are unchanged",
			in: MetricCollectionConfig{
				JobInterval:      time.Hour,
				JobTimeout:       METRICS_DEFAULT_JOB_TIMEOUT,
				CollectorTimeout: METRICS_DEFAULT_COLLECTOR_TIMEOUT,
			},
			expected: MetricCollectionConfig{
				JobInterval:      time.Hour,
				JobTimeout:       10 * time.Minute,
				CollectorTimeout: 2 * time.Minute,
			},
		},
		{
			name: "job timeout longer than the interval is kept",
			in: MetricCollectionConfig{
				JobInterval:      5 * time.Minute,
				JobTimeout:       10 * time.Minute,
				CollectorTimeout: 2 * time.Minute,
			},
			expected: MetricCollectionConfig{
				JobInterval:      5 * time.Minute,
				JobTimeout:       10 * time.Minute,
				CollectorTimeout: 2 * time.Minute,
			},
		},
		{
			name: "job timeout below the minimum is raised to send and save timeouts plus the minimum budget",
			in: MetricCollectionConfig{
				JobInterval:      time.Hour,
				JobTimeout:       time.Minute,
				CollectorTimeout: 2 * time.Minute,
			},
			expected: MetricCollectionConfig{
				JobInterval:      time.Hour,
				JobTimeout:       2 * time.Minute,
				CollectorTimeout: METRICS_MIN_COLLECTION_BUDGET,
			},
		},
		{
			name: "collector timeout above the collection budget is clamped to the budget",
			in: MetricCollectionConfig{
				JobInterval:      time.Hour,
				JobTimeout:       3 * time.Minute,
				CollectorTimeout: 2 * time.Minute,
			},
			expected: MetricCollectionConfig{
				JobInterval:      time.Hour,
				JobTimeout:       3 * time.Minute,
				CollectorTimeout: 90 * time.Second,
			},
		},
		{
			name: "zero values fall back to defaults",
			in: MetricCollectionConfig{
				JobInterval:      time.Hour,
				JobTimeout:       0,
				CollectorTimeout: 0,
			},
			expected: MetricCollectionConfig{
				JobInterval:      time.Hour,
				JobTimeout:       10 * time.Minute,
				CollectorTimeout: 2 * time.Minute,
			},
		},
		{
			name: "negative durations fall back to defaults",
			in: MetricCollectionConfig{
				JobInterval:      time.Hour,
				JobTimeout:       -5 * time.Minute,
				CollectorTimeout: -time.Second,
			},
			expected: MetricCollectionConfig{
				JobInterval:      time.Hour,
				JobTimeout:       10 * time.Minute,
				CollectorTimeout: 2 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.in
			cfg.ValidateAndFix(metricsTestContext())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestMetricCollectionConfig_CollectionBudget(t *testing.T) {
	tests := []struct {
		name       string
		jobTimeout time.Duration
		expected   time.Duration
	}{
		{name: "default job timeout", jobTimeout: 10 * time.Minute, expected: 8*time.Minute + 30*time.Second},
		{name: "job timeout equal to send plus save timeout leaves no budget", jobTimeout: METRICS_SEND_TIMEOUT + METRICS_SAVE_TIMEOUT, expected: 0},
		{name: "job timeout below send timeout gives negative budget", jobTimeout: 30 * time.Second, expected: -time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := MetricCollectionConfig{JobTimeout: tt.jobTimeout}
			assert.Equal(t, tt.expected, cfg.CollectionBudget())
		})
	}
}
