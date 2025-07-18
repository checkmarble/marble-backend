package models

import (
	"time"

	"github.com/google/uuid"
)

type MetricData struct {
	Name        string
	Numeric     *float64
	Text        *string
	Timestamp   time.Time
	PublicOrgID *uuid.UUID // Only for org-specific metrics, use the public id of the org
	From        time.Time
	To          time.Time
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

// Create a new organization metric
// Use the public id of the org to identify the org and not the internal ID
func NewOrganizationMetric(name string, numeric *float64, text *string, publicOrgId uuid.UUID,
	from, to time.Time,
) MetricData {
	return MetricData{
		Name:        name,
		Numeric:     numeric,
		Text:        text,
		Timestamp:   time.Now(),
		PublicOrgID: &publicOrgId,
		From:        from,
		To:          to,
	}
}
