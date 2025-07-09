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
	OrganizationID *string    // Only for org-specific metrics
	From           *time.Time // Optional start time for time range metrics
	To             *time.Time // Optional end time for time range metrics
}

type MetricsCollection struct {
	CollectionID uuid.UUID // Unique ID for this collection run, could be use as idempotency key
	Timestamp    time.Time
	Metrics      []MetricData
	Version      string
	DeploymentID uuid.UUID
	LicenseKey   *string
}

// Bigquery schema for metrics
type MetricEventRow struct {
	StartTime    time.Time            `bigquery:"start_time"`
	EndTime      time.Time            `bigquery:"end_time"`
	DeploymentID uuid.UUID            `bigquery:"deployment_id"`
	LicenseKey   bigquery.NullString  `bigquery:"license_key"`
	OrgID        bigquery.NullString  `bigquery:"org_id"`
	EventType    string               `bigquery:"event_type"`
	Counter      bigquery.NullFloat64 `bigquery:"counter"`
	Gauge        bigquery.NullFloat64 `bigquery:"gauge"`
	Text         bigquery.NullString  `bigquery:"text"`
}

func AdaptMetricsCollection(metricsCollection MetricsCollection) []*MetricEventRow {
	metricEventRows := make([]*MetricEventRow, 0, len(metricsCollection.Metrics))

	licenseKey := pure_utils.NullStringFromPtr(metricsCollection.LicenseKey)

	for _, metric := range metricsCollection.Metrics {
		startTime := metricsCollection.Timestamp
		if metric.From != nil {
			startTime = *metric.From
		}

		endTime := metricsCollection.Timestamp
		if metric.To != nil {
			endTime = *metric.To
		}

		metricEventRows = append(metricEventRows, &MetricEventRow{
			StartTime:    startTime,
			EndTime:      endTime,
			DeploymentID: metricsCollection.DeploymentID,
			LicenseKey:   licenseKey,
			OrgID:        pure_utils.NullStringFromPtr(metric.OrganizationID),
			EventType:    metric.Name,
			Counter:      pure_utils.NullFloat64FromPtr(metric.Numeric),
			Gauge:        bigquery.NullFloat64{},
			Text:         pure_utils.NullStringFromPtr(metric.Text),
		})
	}

	return metricEventRows
}

func NewGlobalMetric(name string, numeric *float64, text *string, from, to *time.Time) MetricData {
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
	from, to *time.Time,
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
