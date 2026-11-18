package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A task whose own timeout is longer than what the job context allows ends on
// its limit (at most half of the remaining job time), not on the job deadline.
func TestAgent_TimeoutIsClampedToHalfTheParentDeadline(t *testing.T) {
	scripted := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		FinalStep(agFinal(agNomFillValid)),
	)
	llm := slowLLM{inner: scripted, delay: 700 * time.Millisecond}
	spec := agentSpec("fill", agentPersonPath)
	spec.Limits.TimeoutSeconds = 600

	parent, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()
	r := NewAgentRunner(llm, fakeAgentPrompts{}, AgentLimits{})
	out, err := r.Run(parent, agentToolContext(t), agentTask(spec), spec)
	require.NoError(t, err, "the task ends on a clean limit before the job context expires")
	assert.Equal(t, 1, scripted.Calls(), "no turn once half of the remaining time is used")
	assert.NotEmpty(t, out.Questions)
	assert.NoError(t, parent.Err(), "the job context is still alive")
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
