package models

import (
	"time"

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
