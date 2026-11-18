package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/invopop/jsonschema"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// A4 (fix wave PR 5, verification-jobs item 5): a flagged structured task that
// ends on the turn limit with no usable output fails the job (River job
// cancelled), instead of being done empty.
func TestRunJob_FailJobOnLimitFailsTheJob(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	spec := Spec("place", "")
	spec.ToolNames = []string{"read_object"}
	spec.OutputSchema = &jsonschema.Schema{Type: "object"}
	spec.FailJobOnLimit = true
	f.Format.Waves = [][]core.TaskSpec{{spec}}

	// Real agent loop, scripted provider: it keeps reading past MaxTurns.
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
	)
	o.runner = NewAgentRunner(llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 1})
	w := NewWorker(o, f.Repo, fakeExecFactory{}, time.Minute)

	err := w.Work(ctx, workerJob(f, 1))
	var cancelErr *river.JobCancelError
	require.True(t, errors.As(err, &cancelErr), "the River job is cancelled, got %v", err)

	j := jobOf(t, f)
	require.Equal(t, models.SarJobStatusFailed, j.Status)
	assert.NotEmpty(t, j.Error)
	require.NotEqual(t, models.SarJobStatusDone, f.Repo.Task(t, "place", "").Status,
		"the limited task is not done with an empty output")
}

// Control: the same limit on an unflagged structured task leaves the job done
// (quality limit, nothing applied).
func TestRunJob_LimitWithoutFailJobOnLimitJobIsDone(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	spec := Spec("place", "")
	spec.ToolNames = []string{"read_object"}
	spec.OutputSchema = &jsonschema.Schema{Type: "object"}
	f.Format.Waves = [][]core.TaskSpec{{spec}}

	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
	)
	o.runner = NewAgentRunner(llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 1})

	require.NoError(t, o.RunJob(ctx, f.JobId))
	require.Equal(t, models.SarJobStatusDone, jobOf(t, f).Status)
}

// In a parallel wave, the limit failure of a FailJobOnLimit task must not be
// shadowed by the technical error of another task that finished first: the job
// fails and the River job is cancelled (no retry), and the job error names the
// limit reason.
func TestRunJob_LimitErrorNotShadowedByOtherTaskError(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	f.Format.Waves = [][]core.TaskSpec{{Spec("other", ""), Spec("critical", "")}}

	otherFailed := make(chan struct{})
	var once sync.Once
	f.Runner.On("other", "", Step{
		Before: func() { once.Do(func() { close(otherFailed) }) },
		Err:    errors.New("erreur technique"),
	})
	f.Runner.On("critical", "", Step{
		Before: func() {
			<-otherFailed
			time.Sleep(100 * time.Millisecond) // let the other error be recorded first
		},
		Err: errors.Mark(errors.Newf("%s (%s)", ErrTaskLimitReached.Error(), "nombre maximal de tours atteint"), ErrTaskLimitReached),
	})
	w := NewWorker(o, f.Repo, fakeExecFactory{}, time.Minute)

	err := w.Work(ctx, workerJob(f, 1))
	var cancelErr *river.JobCancelError
	require.True(t, errors.As(err, &cancelErr), "the River job is cancelled, not retried, got %v", err)

	j := jobOf(t, f)
	require.Equal(t, models.SarJobStatusFailed, j.Status)
	assert.Contains(t, j.Error, "limite atteinte")
	assert.Contains(t, j.Error, "nombre maximal de tours atteint", "the job error names the limit reason")
}
