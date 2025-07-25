package bqmodels

import (
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

// Bigquery schema for metrics
type MetricEventRow struct {
	StartTime      time.Time            `bigquery:"start_time"`
	EndTime        time.Time            `bigquery:"end_time"`
	DeploymentID   uuid.UUID            `bigquery:"deployment_id"`
	LicenseKey     bigquery.NullString  `bigquery:"license_key"`
	LicenseKeyName bigquery.NullString  `bigquery:"license_key_name"`
	PublicOrgID    bigquery.NullString  `bigquery:"public_org_id"`
	EventType      string               `bigquery:"event_type"`
	Value          bigquery.NullFloat64 `bigquery:"value"`
	Text           bigquery.NullString  `bigquery:"text"`
}

func AdaptMetricsCollection(metricsCollection models.MetricsCollection) []*MetricEventRow {
	metricEventRows := make([]*MetricEventRow, 0, len(metricsCollection.Metrics))

	licenseKey := pure_utils.BQNullStringFromPtr(metricsCollection.LicenseKey)
	licenseKeyName := pure_utils.BQNullStringFromPtr(metricsCollection.LicenseName)

	for _, metric := range metricsCollection.Metrics {
		var publicOrgID *string
		if metric.PublicOrgID != nil {
			publicOrgID = utils.Ptr(metric.PublicOrgID.String())
		}

		metricEventRows = append(metricEventRows, &MetricEventRow{
			StartTime:      metric.From,
			EndTime:        metric.To,
			DeploymentID:   metricsCollection.DeploymentID,
			LicenseKey:     licenseKey,
			LicenseKeyName: licenseKeyName,
			PublicOrgID:    pure_utils.BQNullStringFromPtr(publicOrgID),
			EventType:      metric.Name,
			Value:          pure_utils.BQNullFloat64FromPtr(metric.Numeric),
			Text:           pure_utils.BQNullStringFromPtr(metric.Text),
		})
	}

	return metricEventRows
}
