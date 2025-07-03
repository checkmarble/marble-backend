package pure_utils

import (
	"fmt"
	"time"
)

// Frequency represents time period frequencies for utility functions
type Frequency string

const (
	FrequencyDaily   Frequency = "daily"
	FrequencyMonthly Frequency = "monthly"
)

// TimeRange represents a time period with start and end times
type TimeRange struct {
	From time.Time
	To   time.Time
}

// SplitTimeRangeByFrequency splits a time range based on frequency using calendar boundaries
func SplitTimeRangeByFrequency(from, to time.Time, frequency Frequency) ([]TimeRange, error) {
	// NOTE: Should return an error in this case?
	if from.After(to) || from.Equal(to) {
		return []TimeRange{{From: from, To: to}}, nil
	}

	// Always split into individual periods aligned to calendar boundaries
	var ranges []TimeRange
	current := from

	for current.Before(to) {
		periodEnd, err := getNextPeriodBoundary(current, frequency)
		if err != nil {
			return nil, err
		}

		if periodEnd.After(to) {
			periodEnd = to
		}

		ranges = append(ranges, TimeRange{
			From: current,
			To:   periodEnd,
		})

		current = periodEnd
	}

	return ranges, nil
}

// getNextPeriodBoundary returns the start of the next calendar period based on the given frequency.
// For daily frequency, it returns the next day at 00:00:00.
// For monthly frequency, it returns the first day of the next month at 00:00:00.
func getNextPeriodBoundary(current time.Time, frequency Frequency) (time.Time, error) {
	switch frequency {
	case FrequencyDaily:
		return time.Date(current.Year(), current.Month(), current.Day()+1, 0, 0, 0, 0, current.Location()), nil
	case FrequencyMonthly:
		return time.Date(current.Year(), current.Month()+1, 1, 0, 0, 0, 0, current.Location()), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported frequency: %s", frequency)
	}
}
