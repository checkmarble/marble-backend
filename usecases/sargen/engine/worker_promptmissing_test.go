package engine

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// A prompt that cannot be read while a task runs ends the River job at once
// (no second attempt), whatever the attempt number.
func TestWorker_PromptMissingFromATaskCancelsWithoutRetry(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	f.Format.Waves = [][]core.TaskSpec{{Spec("a", tgtPP1)}}
	f.Runner.On("a", tgtPP1, Step{Err: promptMissing("prompts/a.md")})

	w := NewWorker(o, f.Repo, fakeExecFactory{}, time.Minute)
	err := w.Work(ctx, workerJob(f, 1))
	var cancel *river.JobCancelError
	require.True(t, errors.As(err, &cancel), "first attempt must already be cancelled, got %v", err)
	assert.Equal(t, models.SarJobStatusFailed, jobOf(t, f).Status)
	assert.Equal(t, 1, f.Runner.Count("a", tgtPP1), "the task ran once")
}
