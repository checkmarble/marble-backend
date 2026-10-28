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

type ByOrgByProviderCounter map[string]map[string]int

// Input of a metrics collection run: the time windows each collector must cover.
type MetricsCollectionRequest struct {
	Now              time.Time            // UTC, upper bound for every collector
	SnapshotFrom     time.Time            // From for global + snapshot (gauge) collectors
	CounterFrom      map[string]time.Time // keyed by CounterCollector.Name(); collectors absent from the map are skipped
	CollectorTimeout time.Duration
}

// Progress of one counter collector during a run, used to advance its watermark.
type MetricsCounterProgress struct {
	CollectorName  string
	From           time.Time
	CompletedUntil time.Time // == From when no period completed
	Err            error     // nil if caught up
}

type MetricsCollectionResult struct {
	Collection MetricsCollection        // unchanged payload type
	Progress   []MetricsCounterProgress // one per counter collector in request, execution order
}
