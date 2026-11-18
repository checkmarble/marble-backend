package engine

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A task that fails the job on a limit and whose own timeout is longer than
// what the job context allows ends on its limit (at most half of the remaining
// job time), not on the job deadline.
func TestAgent_TimeoutIsClampedToHalfTheParentDeadline(t *testing.T) {
	scripted := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		FinalStep(agFinal(agNomFillValid)),
	)
	llm := slowLLM{inner: scripted, delay: 700 * time.Millisecond}
	spec := agentSpec("fill", agentPersonPath)
	spec.Limits.TimeoutSeconds = 600
	spec.FailJobOnLimit = true

	parent, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()
	r := NewAgentRunner(llm, fakeAgentPrompts{}, AgentLimits{})
	out, err := r.Run(parent, agentToolContext(t), agentTask(spec), spec)
	require.True(t, errors.Is(err, ErrTaskLimitReached), "the task ends on its own limit before the job context expires")
	assert.Equal(t, 1, scripted.Calls(), "no turn once half of the remaining time is used")
	assert.Empty(t, out.Fills)
	assert.NoError(t, parent.Err(), "the job context is still alive")
}

// Final review I1, converse: a task that does not fail the job on a limit keeps
// its own budget. When the job context expires first, the run fails technically
// (River retries and the resume redoes the task with its whole budget) instead
// of ending "done" on a clamped limit.
func TestAgent_TimeoutIsNotClampedForOtherTasks(t *testing.T) {
	scripted := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		FinalStep(agFinal(agNomFillValid)),
	)
	llm := ctxSlowLLM{inner: scripted, delay: 700 * time.Millisecond}
	spec := agentSpec("fill", agentPersonPath)
	spec.Limits.TimeoutSeconds = 600

	parent, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()
	r := NewAgentRunner(llm, fakeAgentPrompts{}, AgentLimits{})
	out, err := r.Run(parent, agentToolContext(t), agentTask(spec), spec)
	require.Error(t, err, "the job deadline is a technical failure for this task, not a limit")
	assert.Empty(t, out.Questions, "no limit output: the task is not done")
	assert.Error(t, parent.Err(), "the job context expired")
}

// ctxSlowLLM waits for its delay or for the context, whichever comes first.
type ctxSlowLLM struct {
	inner LLM
	delay time.Duration
}

func (s ctxSlowLLM) Step(ctx context.Context, req AgentRequest) (AgentStep, error) {
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return AgentStep{}, ctx.Err()
	}
	return s.inner.Step(ctx, req)
}

// A per-task limit never lowers the runner's own budget: with the runner at 50
// calls and the spec at 1, a run of 3 tool calls then a final is not cut.
func TestAgent_SpecLimitsOnlyRaiseTheRunnerBudget(t *testing.T) {
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
		FinalStep(agFinal(agNomFillValid)),
	)
	spec := agentSpec("fill", agentPersonPath)
	spec.Limits.MaxToolCalls, spec.Limits.MaxTurns = 1, 1
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxToolCalls: 50, MaxTurns: 50}, agentToolContext(t), spec)
	require.NoError(t, err)
	assert.Equal(t, 4, llm.Calls(), "the run is not cut at the spec's 1 call / 1 turn")
	assert.NotEmpty(t, out.Fills)
}
