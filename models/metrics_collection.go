package models

import (
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/google/uuid"
)

type MetricCollectionFrequency string

const (
	MetricCollectionFrequencyInstant MetricCollectionFrequency = "instant"
	MetricCollectionFrequencyDaily   MetricCollectionFrequency = "daily"
	MetricCollectionFrequencyMonthly MetricCollectionFrequency = "monthly"
)

type MetricData struct {
	Name           string
	Numeric        *float64
	Text           *string
	Timestamp      time.Time
	OrganizationID *string    // Only for org-specific metrics
	From           *time.Time // Optional start time for time range metrics
	To             *time.Time // Optional end time for time range metrics
	Frequency      MetricCollectionFrequency
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

	licenseKey := bigquery.NullString{}
	if metricsCollection.LicenseKey != nil {
		licenseKey.StringVal = *metricsCollection.LicenseKey
		licenseKey.Valid = true
	}

	for _, metric := range metricsCollection.Metrics {
		orgID := bigquery.NullString{}
		if metric.OrganizationID != nil {
			orgID.StringVal = *metric.OrganizationID
			orgID.Valid = true
		}

		counter := bigquery.NullFloat64{}
		if metric.Numeric != nil {
			counter.Float64 = *metric.Numeric
			counter.Valid = true
		}

		text := bigquery.NullString{}
		if metric.Text != nil {
			text.StringVal = *metric.Text
			text.Valid = true
		}

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
			OrgID:        orgID,
			EventType:    metric.Name,
			Counter:      counter,
			Gauge:        bigquery.NullFloat64{},
			Text:         text,
		})
	}

	return metricEventRows
}

func NewGlobalMetric(name string, numeric *float64, text *string, from, to *time.Time, frequency MetricCollectionFrequency) MetricData {
	return MetricData{
		Name:      name,
		Numeric:   numeric,
		Text:      text,
		Timestamp: time.Now(),
		From:      from,
		To:        to,
		Frequency: frequency,
	}
}

func NewOrganizationMetric(name string, numeric *float64, text *string, orgID string,
	from, to *time.Time, frequency MetricCollectionFrequency,
) MetricData {
	return MetricData{
		Name:           name,
		Numeric:        numeric,
		Text:           text,
		Timestamp:      time.Now(),
		OrganizationID: &orgID,
		From:           from,
		To:             to,
		Frequency:      frequency,
	}
}
