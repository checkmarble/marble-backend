package metrics_collection

import "time"

// MetricFrequency defines how often a metric should be collected/split
type MetricFrequency string

const (
	FrequencyInstant MetricFrequency = "instant" // Single point in time
	FrequencyDaily   MetricFrequency = "daily"   // Split by day boundaries
	FrequencyMonthly MetricFrequency = "monthly" // Split by month boundaries
)

type MetricData struct {
	Name      string     `json:"name"`
	Value     any        `json:"value"` // Can be int, string, float, etc.
	Timestamp time.Time  `json:"timestamp"`
	From      *time.Time `json:"from,omitempty"`
	// Optional start time for time range metrics
	To *time.Time `json:"to,omitempty"`
	// Optional end time for time range metrics
	MetricType     string  `json:"type"`                      // "global" or "organization"
	OrganizationID *string `json:"organization_id,omitempty"` // Only for org-specific metrics
	Info           *string `json:"info,omitempty"`            // Optional additional information
}

type MetricsPayload struct {
	CollectionID string       `json:"collection_id"` // Unique ID for this collection run
	Timestamp    time.Time    `json:"timestamp"`     // When collection started
	Metrics      []MetricData `json:"metrics"`
}

// Helper functions for creating metrics

// NewGlobalMetric creates a global metric (not organization-specific)
func NewGlobalMetric(name string, value any) MetricData {
	return MetricData{
		Name:       name,
		Value:      value,
		Timestamp:  time.Now(),
		MetricType: "global",
	}
}

// NewOrganizationMetric creates an organization-specific metric
func NewOrganizationMetric(name string, value any, orgID string) MetricData {
	return MetricData{
		Name:           name,
		Value:          value,
		Timestamp:      time.Now(),
		MetricType:     "organization",
		OrganizationID: &orgID,
	}
}

// NewGlobalMetricWithTimeRange creates a global metric with time range
func NewGlobalMetricWithTimeRange(name string, value any, from, to time.Time) MetricData {
	return MetricData{
		Name:       name,
		Value:      value,
		Timestamp:  time.Now(),
		From:       &from,
		To:         &to,
		MetricType: "global",
	}
}

// NewOrganizationMetricWithTimeRange creates an organization-specific metric with time range
func NewOrganizationMetricWithTimeRange(name string, value any, orgID string, from, to time.Time) MetricData {
	return MetricData{
		Name:           name,
		Value:          value,
		Timestamp:      time.Now(),
		From:           &from,
		To:             &to,
		MetricType:     "organization",
		OrganizationID: &orgID,
	}
}

// WithInfo adds optional info to a metric
func (m MetricData) WithInfo(info string) MetricData {
	m.Info = &info
	return m
}

// WithTimeRange adds time range to a metric
func (m MetricData) WithTimeRange(from, to time.Time) MetricData {
	m.From = &from
	m.To = &to
	return m
}

// Time range splitting functions

// SplitMetricByFrequency splits a metric with time range according to the specified frequency
func SplitMetricByFrequency(baseMetric MetricData, frequency MetricFrequency,
	valueCalculator func(from, to time.Time) any,
) []MetricData {
	if baseMetric.From == nil || baseMetric.To == nil {
		// No time range, return as single metric
		return []MetricData{baseMetric}
	}

	switch frequency {
	case FrequencyDaily:
		return splitMetricByDay(baseMetric, valueCalculator)
	case FrequencyMonthly:
		return splitMetricByMonth(baseMetric, valueCalculator)
	default:
		return []MetricData{baseMetric}
	}
}

// splitMetricByDay splits a metric by day boundaries
func splitMetricByDay(baseMetric MetricData, valueCalculator func(from, to time.Time) any) []MetricData {
	var metrics []MetricData

	from := *baseMetric.From
	to := *baseMetric.To

	current := from
	for current.Before(to) {
		// Find the end of current day or the end time, whichever is earlier
		endOfDay := time.Date(current.Year(), current.Month(), current.Day(), 23, 59, 59, 999999999, current.Location())
		periodEnd := to
		if endOfDay.Before(to) {
			periodEnd = endOfDay
		}

		// Calculate value for this period
		value := baseMetric.Value
		if valueCalculator != nil {
			value = valueCalculator(current, periodEnd)
		}

		// Create metric for this period
		metric := baseMetric
		metric.From = &current
		metric.To = &periodEnd
		metric.Value = value
		metric.Timestamp = time.Now()

		metrics = append(metrics, metric)

		// Move to start of next day
		current = time.Date(current.Year(), current.Month(), current.Day()+1, 0, 0, 0, 0, current.Location())
	}

	return metrics
}

// splitMetricByMonth splits a metric by month boundaries
func splitMetricByMonth(baseMetric MetricData, valueCalculator func(from, to time.Time) any) []MetricData {
	var metrics []MetricData

	from := *baseMetric.From
	to := *baseMetric.To

	current := from
	for current.Before(to) {
		// Find the end of current month or the end time, whichever is earlier
		year, month, _ := current.Date()
		endOfMonth := time.Date(year, month+1, 0, 23, 59, 59, 999999999, current.Location())
		periodEnd := to
		if endOfMonth.Before(to) {
			periodEnd = endOfMonth
		}

		// Calculate value for this period
		value := baseMetric.Value
		if valueCalculator != nil {
			value = valueCalculator(current, periodEnd)
		}

		// Create metric for this period
		metric := baseMetric
		metric.From = &current
		metric.To = &periodEnd
		metric.Value = value
		metric.Timestamp = time.Now()

		metrics = append(metrics, metric)

		// Move to start of next month
		current = time.Date(year, month+1, 1, 0, 0, 0, 0, current.Location())
	}

	return metrics
}

// Helper function for collectors to easily create time-split metrics
func CreateTimeRangeMetrics(name string, from, to time.Time, frequency MetricFrequency,
	isGlobal bool, orgID string, valueCalculator func(from, to time.Time) any,
) []MetricData {
	var baseMetric MetricData

	if isGlobal {
		baseMetric = NewGlobalMetricWithTimeRange(name, nil, from, to)
	} else {
		baseMetric = NewOrganizationMetricWithTimeRange(name, nil, orgID, from, to)
	}

	return SplitMetricByFrequency(baseMetric, frequency, valueCalculator)
}
