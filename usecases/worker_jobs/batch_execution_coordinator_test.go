package worker_jobs

import (
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
)

func TestBatchExecutionCoordinatorNextRetry(t *testing.T) {
	worker := NewBatchExecutionCoordinatorWorker(nil)

	tts := []struct {
		previousErrors int
		expected       time.Duration
	}{
		{0, 5 * time.Second},
		{1, 10 * time.Second},
		{2, 20 * time.Second},
		{4, 80 * time.Second},
		{8, 1280 * time.Second},
		{9, 30 * time.Minute},
		{20, 30 * time.Minute},
		{199, 30 * time.Minute},
	}

	for _, tt := range tts {
		job := &river.Job[models.BatchExecutionCoordinatorArgs]{
			JobRow: &rivertype.JobRow{Errors: make([]rivertype.AttemptError, tt.previousErrors)},
		}

		before := time.Now()
		delay := worker.NextRetry(job).Sub(before)

		assert.InDelta(t, tt.expected, delay, float64(time.Second), "after %d errors", tt.previousErrors)
	}
}

func TestBatchExecBackoffDelayInLoopCap(t *testing.T) {
	assert.Equal(t, 5*time.Second, batchExecBackoffDelay(0, batchExecBackoffCap))
	assert.Equal(t, 5*time.Second, batchExecBackoffDelay(1, batchExecBackoffCap))
	assert.Equal(t, 80*time.Second, batchExecBackoffDelay(5, batchExecBackoffCap))
	assert.Equal(t, 2*time.Minute, batchExecBackoffDelay(6, batchExecBackoffCap))
	assert.Equal(t, 2*time.Minute, batchExecBackoffDelay(100, batchExecBackoffCap))
}
