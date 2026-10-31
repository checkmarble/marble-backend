package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEffectiveScheduledExecutionStatus(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Second)
	future := now.Add(time.Minute)

	tests := []struct {
		name     string
		status   ScheduledExecutionStatus
		deadline *time.Time
		expected ScheduledExecutionStatus
	}{
		{"cancelling before deadline", ScheduledExecutionCancelling, &future, ScheduledExecutionCancelling},
		{"cancelling after deadline", ScheduledExecutionCancelling, &past, ScheduledExecutionCancelled},
		{"cancelling without deadline", ScheduledExecutionCancelling, nil, ScheduledExecutionCancelling},
		{"processing with past deadline", ScheduledExecutionProcessing, &past, ScheduledExecutionProcessing},
		{"success with past deadline", ScheduledExecutionSuccess, &past, ScheduledExecutionSuccess},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectiveScheduledExecutionStatus(tt.status, tt.deadline, now)

			assert.Equal(t, tt.expected, got)

			if tt.expected == ScheduledExecutionCancelled {
				assert.False(t, got.IsActive())
			}
		})
	}
}
