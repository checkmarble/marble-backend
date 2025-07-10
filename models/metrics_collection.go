package models

import (
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type MetricData struct {
	Name           string
	Numeric        *float64
	Text           *string
	Timestamp      time.Time
	OrganizationID *string // Only for org-specific metrics
	From           time.Time
	To             time.Time
}

type MetricsCollection struct {
	CollectionID uuid.UUID // Unique ID for this collection run, could be use as idempotency key
	Timestamp    time.Time
	Metrics      []MetricData
	Version      string
	DeploymentID uuid.UUID
	LicenseKey   *string
	LicenseName  *string
}

// Bigquery schema for metrics
type MetricEventRow struct {
	StartTime      time.Time            `bigquery:"start_time"`
	EndTime        time.Time            `bigquery:"end_time"`
	DeploymentID   uuid.UUID            `bigquery:"deployment_id"`
	LicenseKey     bigquery.NullString  `bigquery:"license_key"`
	LicenseKeyName bigquery.NullString  `bigquery:"license_key_name"`
	OrgID          bigquery.NullString  `bigquery:"org_id"`
	EventType      string               `bigquery:"event_type"`
	Value          bigquery.NullFloat64 `bigquery:"value"`
	Text           bigquery.NullString  `bigquery:"text"`
}

func AdaptMetricsCollection(metricsCollection MetricsCollection) []*MetricEventRow {
	metricEventRows := make([]*MetricEventRow, 0, len(metricsCollection.Metrics))

	licenseKey := pure_utils.BQNullStringFromPtr(metricsCollection.LicenseKey)
	licenseKeyName := pure_utils.BQNullStringFromPtr(metricsCollection.LicenseName)

	for _, metric := range metricsCollection.Metrics {
		metricEventRows = append(metricEventRows, &MetricEventRow{
			StartTime:      metric.From,
			EndTime:        metric.To,
			DeploymentID:   metricsCollection.DeploymentID,
			LicenseKey:     licenseKey,
			LicenseKeyName: licenseKeyName,
			OrgID:          pure_utils.BQNullStringFromPtr(metric.OrganizationID),
			EventType:      metric.Name,
			Value:          pure_utils.BQNullFloat64FromPtr(metric.Numeric),
			Text:           pure_utils.BQNullStringFromPtr(metric.Text),
		})
	}

	return metricEventRows
}

func NewGlobalMetric(name string, numeric *float64, text *string, from, to time.Time) MetricData {
	return MetricData{
		Name:      name,
		Numeric:   numeric,
		Text:      text,
		Timestamp: time.Now(),
		From:      from,
		To:        to,
	}
}

func NewOrganizationMetric(name string, numeric *float64, text *string, orgID string,
	from, to time.Time,
) MetricData {
	return MetricData{
		Name:           name,
		Numeric:        numeric,
		Text:           text,
		Timestamp:      time.Now(),
		OrganizationID: &orgID,
		From:           from,
		To:             to,
	}
}
