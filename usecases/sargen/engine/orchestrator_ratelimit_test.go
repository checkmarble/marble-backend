package engine

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

func TestRunJob_RateLimitCappedAfterTwoHours(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	f.Format.Waves = [][]core.TaskSpec{{Spec("a", tgtPP1)}}
	f.Runner.On("a", tgtPP1, Step{Err: RateLimited()})
	w := NewWorker(o, f.Repo, fakeExecFactory{}, time.Minute)

	// First hit: the streak starts, the job snoozes.
	err := w.Work(ctx, workerJob(f, 1))
	var snooze *river.JobSnoozeError
	require.True(t, errors.As(err, &snooze), "got %v", err)
	require.NotNil(t, jobOf(t, f).RateLimitedSince)

	// Still within the window.
	f.Advance(RateLimitMaxDuration - time.Minute)
	err = w.Work(ctx, workerJob(f, 1))
	require.True(t, errors.As(err, &snooze), "got %v", err)
	require.Equal(t, models.SarJobStatusRunning, jobOf(t, f).Status)

	// Past 2 h: failed with the French message, River job cancelled.
	f.Advance(2 * time.Minute)
	err = w.Work(ctx, workerJob(f, 1))
	var cancelErr *river.JobCancelError
	require.True(t, errors.As(err, &cancelErr), "got %v", err)
	j := jobOf(t, f)
	require.Equal(t, models.SarJobStatusFailed, j.Status)
	require.Equal(t, "limite de débit du fournisseur LLM dépassée trop longtemps", j.Error)
}

func TestRunJob_RateLimitStreakResetsWhenATaskSucceeds(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	f.Format.Waves = [][]core.TaskSpec{{Spec("a", tgtPP1)}}
	f.Runner.On("a", tgtPP1, Step{Err: RateLimited()}, Step{})

	require.True(t, errors.Is(o.RunJob(ctx, f.JobId), models.LLMRateLimitedError))
	require.NotNil(t, jobOf(t, f).RateLimitedSince)

	f.Advance(RateLimitMaxDuration + time.Hour) // would exceed the cap, but the task now succeeds
	require.NoError(t, o.RunJob(ctx, f.JobId))
	j := jobOf(t, f)
	require.Equal(t, models.SarJobStatusDone, j.Status)
	require.Nil(t, j.RateLimitedSince)
}

func TestRunJob_ModelNotConfiguredFailsJobAndCancelsRiverJob(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	f.Format.Waves = [][]core.TaskSpec{{Spec("a", tgtPP1)}}
	f.Runner.On("a", tgtPP1, Step{Err: errors.Mark(errors.New("aucun modèle configuré pour la génération (niveau light)"), ErrLLMUnavailable)})
	w := NewWorker(o, f.Repo, fakeExecFactory{}, time.Minute)

	err := w.Work(ctx, workerJob(f, 1))
	var cancelErr *river.JobCancelError
	require.True(t, errors.As(err, &cancelErr), "got %v", err)
	j := jobOf(t, f)
	require.Equal(t, models.SarJobStatusFailed, j.Status)
	require.Equal(t, "génération impossible : aucun modèle configuré pour la génération (niveau light)", j.Error)
}

func TestLlmberjackLLM_ConfigErrorsAreNonRetryable(t *testing.T) {
	_, err := NewLlmberjackLLM(nil, nil).Step(context.Background(), AgentRequest{RunID: "r", Tier: core.TierLight})
	require.True(t, errors.Is(err, ErrLLMUnavailable))
	require.Contains(t, err.Error(), "non configuré")
}

func structuredAdding(ref core.ObjectRef) func(string, *core.Document, json.RawMessage) error {
	return func(_ string, doc *core.Document, _ json.RawMessage) error {
		p := doc.AddItem("personnes.pp", "pp")
		doc.ItemSources[p] = []core.ObjectRef{ref}
		return nil
	}
}

func TestRunJob_StructuredOutputOutsideSelectionIsRejectedWhole(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	f.Format.Waves = [][]core.TaskSpec{{Spec("place", "")}}
	f.Format.StructuredFn = structuredAdding(core.ObjectRef{Table: "persons", ObjectID: "stranger"})
	f.Runner.On("place", "", Step{Out: core.TaskOutput{Structured: json.RawMessage(`{"x":1}`)}})
	f.Repo.selection = []models.SarSelectedObject{{TableName: "persons", ObjectId: "p1", Snapshot: map[string]any{}}}

	require.NoError(t, o.RunJob(ctx, f.JobId))
	require.Empty(t, f.Repo.Doc(t).Items["personnes.pp"], "nothing of the structured output is applied")
	out := string(f.Repo.Task(t, "place", "").Output)
	require.Contains(t, out, "objet hors sélection : persons/stranger")
	require.Contains(t, out, core.CodeSourceOutOfScope)
}

func TestRunJob_StructuredOutputInsideSelectionIsApplied(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	f.Format.Waves = [][]core.TaskSpec{{Spec("place", "")}}
	f.Format.StructuredFn = structuredAdding(core.ObjectRef{Table: "persons", ObjectID: "p1"})
	f.Runner.On("place", "", Step{Out: core.TaskOutput{Structured: json.RawMessage(`{"x":1}`)}})
	f.Repo.selection = []models.SarSelectedObject{{TableName: "persons", ObjectId: "p1", Snapshot: map[string]any{}}}

	require.NoError(t, o.RunJob(ctx, f.JobId))
	require.Len(t, f.Repo.Doc(t).Items["personnes.pp"], 1)
}
