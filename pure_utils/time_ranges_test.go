package pure_utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitTimeRangeByFrequency_Daily(t *testing.T) {
	tests := []struct {
		name     string
		from     time.Time
		to       time.Time
		expected []TimeRange
	}{
		{
			name: "single day range",
			from: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
			to:   time.Date(2023, 1, 15, 18, 45, 0, 0, time.UTC),
			expected: []TimeRange{
				{
					From: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
					To:   time.Date(2023, 1, 15, 18, 45, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "two day range",
			from: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
			to:   time.Date(2023, 1, 16, 18, 45, 0, 0, time.UTC),
			expected: []TimeRange{
				{
					From: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
					To:   time.Date(2023, 1, 16, 0, 0, 0, 0, time.UTC),
				},
				{
					From: time.Date(2023, 1, 16, 0, 0, 0, 0, time.UTC),
					To:   time.Date(2023, 1, 16, 18, 45, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "three day range crossing month boundary",
			from: time.Date(2023, 1, 30, 14, 0, 0, 0, time.UTC),
			to:   time.Date(2023, 2, 1, 12, 0, 0, 0, time.UTC),
			expected: []TimeRange{
				{
					From: time.Date(2023, 1, 30, 14, 0, 0, 0, time.UTC),
					To:   time.Date(2023, 1, 31, 0, 0, 0, 0, time.UTC),
				},
				{
					From: time.Date(2023, 1, 31, 0, 0, 0, 0, time.UTC),
					To:   time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					From: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
					To:   time.Date(2023, 2, 1, 12, 0, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "range crossing year boundary",
			from: time.Date(2023, 12, 31, 20, 0, 0, 0, time.UTC),
			to:   time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC),
			expected: []TimeRange{
				{
					From: time.Date(2023, 12, 31, 20, 0, 0, 0, time.UTC),
					To:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					From: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					To:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
				},
				{
					From: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					To:   time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SplitTimeRangeByFrequency(tt.from, tt.to, FrequencyDaily)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSplitTimeRangeByFrequency_Monthly(t *testing.T) {
	tests := []struct {
		name     string
		from     time.Time
		to       time.Time
		expected []TimeRange
	}{
		{
			name: "single month range",
			from: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
			to:   time.Date(2023, 1, 25, 18, 45, 0, 0, time.UTC),
			expected: []TimeRange{
				{
					From: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
					To:   time.Date(2023, 1, 25, 18, 45, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "two month range",
			from: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
			to:   time.Date(2023, 2, 10, 18, 45, 0, 0, time.UTC),
			expected: []TimeRange{
				{
					From: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
					To:   time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					From: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
					To:   time.Date(2023, 2, 10, 18, 45, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "range crossing year boundary",
			from: time.Date(2023, 11, 15, 10, 0, 0, 0, time.UTC),
			to:   time.Date(2024, 2, 5, 14, 0, 0, 0, time.UTC),
			expected: []TimeRange{
				{
					From: time.Date(2023, 11, 15, 10, 0, 0, 0, time.UTC),
					To:   time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					From: time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
					To:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					From: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					To:   time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					From: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
					To:   time.Date(2024, 2, 5, 14, 0, 0, 0, time.UTC),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SplitTimeRangeByFrequency(tt.from, tt.to, FrequencyMonthly)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSplitTimeRangeByFrequency_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		from      time.Time
		to        time.Time
		frequency Frequency
		expected  []TimeRange
	}{
		{
			name:      "from after to",
			from:      time.Date(2023, 1, 20, 10, 0, 0, 0, time.UTC),
			to:        time.Date(2023, 1, 15, 10, 0, 0, 0, time.UTC),
			frequency: FrequencyDaily,
			expected: []TimeRange{
				{
					From: time.Date(2023, 1, 20, 10, 0, 0, 0, time.UTC),
					To:   time.Date(2023, 1, 15, 10, 0, 0, 0, time.UTC),
				},
			},
		},
		{
			name:      "from equal to to",
			from:      time.Date(2023, 1, 15, 10, 0, 0, 0, time.UTC),
			to:        time.Date(2023, 1, 15, 10, 0, 0, 0, time.UTC),
			frequency: FrequencyDaily,
			expected: []TimeRange{
				{
					From: time.Date(2023, 1, 15, 10, 0, 0, 0, time.UTC),
					To:   time.Date(2023, 1, 15, 10, 0, 0, 0, time.UTC),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SplitTimeRangeByFrequency(tt.from, tt.to, tt.frequency)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSplitTimeRangeByFrequency_UnknownFrequency(t *testing.T) {
	from := time.Date(2023, 1, 15, 10, 0, 0, 0, time.UTC)
	to := time.Date(2023, 1, 17, 10, 0, 0, 0, time.UTC)

	result, err := SplitTimeRangeByFrequency(from, to, Frequency("unknown"))

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported frequency: unknown")
}

func TestGetNextPeriodBoundary_Daily(t *testing.T) {
	tests := []struct {
		name     string
		current  time.Time
		expected time.Time
	}{
		{
			name:     "middle of day",
			current:  time.Date(2023, 1, 15, 14, 30, 45, 0, time.UTC),
			expected: time.Date(2023, 1, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "end of month",
			current:  time.Date(2023, 1, 31, 23, 59, 59, 0, time.UTC),
			expected: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "end of year",
			current:  time.Date(2023, 12, 31, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "leap year february",
			current:  time.Date(2024, 2, 28, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "leap year february end",
			current:  time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "non-leap year february end",
			current:  time.Date(2023, 2, 28, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "april 30th to may 1st",
			current:  time.Date(2023, 4, 30, 15, 30, 0, 0, time.UTC),
			expected: time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getNextPeriodBoundary(tt.current, FrequencyDaily)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetNextPeriodBoundary_Monthly(t *testing.T) {
	tests := []struct {
		name     string
		current  time.Time
		expected time.Time
	}{
		{
			name:     "middle of january",
			current:  time.Date(2023, 1, 15, 14, 30, 45, 0, time.UTC),
			expected: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "end of february",
			current:  time.Date(2023, 2, 28, 23, 59, 59, 0, time.UTC),
			expected: time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "december to january next year",
			current:  time.Date(2023, 12, 15, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "leap year february",
			current:  time.Date(2024, 2, 15, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "november to december",
			current:  time.Date(2023, 11, 30, 23, 59, 59, 0, time.UTC),
			expected: time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getNextPeriodBoundary(tt.current, FrequencyMonthly)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetNextPeriodBoundary_UnknownFrequency(t *testing.T) {
	current := time.Date(2023, 1, 15, 14, 30, 45, 0, time.UTC)
	result, err := getNextPeriodBoundary(current, Frequency("unknown"))

	assert.Error(t, err)
	assert.True(t, result.IsZero())
	assert.Contains(t, err.Error(), "unsupported frequency: unknown")
}

// TestSplitTimeRangeByFrequency_WithTimezone tests the functions with different timezones
func TestSplitTimeRangeByFrequency_WithTimezone(t *testing.T) {
	// Test with EST timezone
	est, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	from := time.Date(2023, 1, 15, 20, 0, 0, 0, est)
	to := time.Date(2023, 1, 16, 10, 0, 0, 0, est)

	result, err := SplitTimeRangeByFrequency(from, to, FrequencyDaily)
	require.NoError(t, err)

	expected := []TimeRange{
		{
			From: time.Date(2023, 1, 15, 20, 0, 0, 0, est),
			To:   time.Date(2023, 1, 16, 0, 0, 0, 0, est),
		},
		{
			From: time.Date(2023, 1, 16, 0, 0, 0, 0, est),
			To:   time.Date(2023, 1, 16, 10, 0, 0, 0, est),
		},
	}

	assert.Equal(t, expected, result)
}
