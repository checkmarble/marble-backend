package engine

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// A timeout cancels ctx: the task must still end up failed (claimable), not running.
func TestRunJob_CancelledContextLeavesTaskFailed(t *testing.T) {
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	ctx, cancel := context.WithCancel(context.Background())

	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Runner.On("a", tgtPP1, Step{Err: context.DeadlineExceeded, Before: cancel})

	err := o.RunJob(ctx, f.JobId)
	require.Error(t, err)
	require.Equal(t, models.SarJobStatusFailed, f.Repo.Task(t, "a", tgtPP1).Status)

	// Next River attempt reclaims it.
	f.Runner.On("a", tgtPP1, Step{Out: outputWith(Fill(pathNomPP1, "DUPONT"))})
	require.NoError(t, o.RunJob(context.Background(), f.JobId))
	require.Equal(t, models.SarJobStatusDone, f.Repo.Task(t, "a", tgtPP1).Status)
}

// Tasks running elsewhere: no failure, the worker snoozes.
func TestWorker_TasksRunningElsewhereSnoozes(t *testing.T) {
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Runner.On("a", tgtPP1, Step{Err: errProvider})
	require.Error(t, o.RunJob(context.Background(), f.JobId))
	f.Repo.SetTask(t, "a", tgtPP1, func(x *models.SarGenerationTask) {
		x.Status = models.SarJobStatusRunning
		started := f.Now()
		x.StartedAt = &started
	})

	w := NewWorker(o, f.Repo, fakeExecFactory{}, time.Minute)
	err := w.Work(context.Background(), workerJob(f, models.SarGenerationMaxAttempts))
	var snooze *river.JobSnoozeError
	require.True(t, errors.As(err, &snooze), "got %v", err)
	require.Equal(t, RunningElsewhereSnooze, snooze.Duration)
	require.Equal(t, models.SarJobStatusRunning, jobOf(t, f).Status)
}

func TestWorker_MissingPromptCancelsRiverJob(t *testing.T) {
	o, f := newTestOrchestrator(t)
	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Prompts.Missing["prompts/a.md"] = true

	w := NewWorker(o, f.Repo, fakeExecFactory{}, time.Minute)
	err := w.Work(context.Background(), workerJob(f, 1))
	var cancel *river.JobCancelError
	require.True(t, errors.As(err, &cancel), "got %v", err)
	require.Equal(t, models.SarJobStatusFailed, jobOf(t, f).Status)
}

func TestRunJob_RunnerPanicFailsTask(t *testing.T) {
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Runner.On("a", tgtPP1, Step{Before: func() { panic("boom") }})

	err := o.RunJob(context.Background(), f.JobId)
	require.Error(t, err)
	require.Equal(t, models.SarJobStatusFailed, f.Repo.Task(t, "a", tgtPP1).Status)
}
