package models

import (
	"time"

	"github.com/google/uuid"
)

type MetricData struct {
	Name      string
	Numeric   *float64
	Text      *string
	Timestamp time.Time
	OrgID     *string // Only for org-specific metrics
	From      time.Time
	To        time.Time
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
		Name:      name,
		Numeric:   numeric,
		Text:      text,
		Timestamp: time.Now(),
		OrgID:     &orgID,
		From:      from,
		To:        to,
	}
}
