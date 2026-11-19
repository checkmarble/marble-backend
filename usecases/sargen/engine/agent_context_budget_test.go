package engine

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/checkmarble/llmberjack"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// ---------- D: a prepared answer is never lost ----------

const agValidateNomArgs = `{"fills":[{"path":"` + agNomPath + `","value":"Dupont"}]}`

func TestAgent_GraceTurnIsSentWithoutTools(t *testing.T) {
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
		FinalStep(agFinal(agFill(agNomPath, "Dupont", agSrc("last_name", "Dupont")))),
	)
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 2}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)

	require.Equal(t, 3, llm.Calls())
	assert.NotEmpty(t, llm.Requests[0].Tools)
	assert.NotEmpty(t, llm.Requests[1].Tools)
	assert.Empty(t, llm.Requests[2].Tools, "the grace turn may only answer")
	last := llm.Requests[2].History
	assert.Equal(t, budgetMessage, last[len(last)-1].Text)
	require.Len(t, out.Fills, 1, "the answer given on the grace turn is kept")
}

func TestAgent_ValidateWithFillsOnTheLastStepIsTheFinalAnswer(t *testing.T) {
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("validate", agValidateNomArgs), // the model ignores the missing tools
	)
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 1}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)

	assert.Equal(t, 2, llm.Calls(), "no further turn after the validate call")
	// Without a source the copied value cannot be applied: it goes through the
	// normal control, and the unverifiable value becomes a question, not a
	// generic limit question.
	require.Len(t, out.Questions, 1)
	assert.Contains(t, out.Questions[0].Text, "valeur non vérifiable")
	assert.Equal(t, []core.Path{agNomPath}, out.Questions[0].Paths)
	assert.NotContains(t, out.Questions[0].Text, "à renseigner manuellement")
}

func TestAgent_NothingPreparedKeepsTheLimitQuestion(t *testing.T) {
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
	)
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 1}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)

	assert.Empty(t, out.Fills)
	require.NotEmpty(t, out.Questions)
	assert.Contains(t, out.Questions[0].Text, "à renseigner manuellement")
}

func TestBuildTurnRequest_NoToolsOnTheGraceTurn(t *testing.T) {
	thread := &llmberjack.ThreadId{}
	st := turnTwoRun(thread, llmberjack.ResponseToolCall{Id: "1", Name: "read_object", Parameters: []byte(`{"table":"persons","object_id":"p1"}`)})
	c1 := ToolCall{ID: "1", Name: "read_object"}
	req := turnTwoRequest(t, AgentMessage{Role: RoleTool, Text: "result", ToolCall: &c1},
		AgentMessage{Role: RoleUser, Text: budgetMessage})
	req.Tools = nil
	r, err := buildTurnRequest(st, req, "main", "m")
	require.NoError(t, err)

	assert.Empty(t, r.Tools, "no tool definition is sent")
	require.Len(t, r.Messages, 2, "the result of the previous call and the budget message are still sent")
	assert.Equal(t, "result", readParts(t, r.Messages[0]))
}

// ---------- C: task objects inline in the prompt ----------

var nonceRe = regexp.MustCompile(`id="[0-9a-f]{16}"`)

func promptOf(t *testing.T, tc core.ToolContext, spec core.TaskSpec) string {
	t.Helper()
	llm := newScriptedLLM(FinalStep(agFinal()))
	_, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, tc, spec)
	require.NoError(t, err)
	return llm.Requests[0].Prompt
}

func txnAccountContext(t *testing.T) (core.ToolContext, core.TaskSpec) {
	t.Helper()
	tc := agentToolContext(t,
		agentSelection("transactions", "txn-1", map[string]any{"object_id": "txn-1", "amount": 1200, "account_id": "acc-1"}),
		agentSelection("transactions", "txn-2", map[string]any{"object_id": "txn-2", "amount": 55, "account_id": "acc-1"}),
		agentSelection("accounts", "acc-1", map[string]any{"object_id": "acc-1", "name": "Compte courant ACME"}),
		agentSelection("accounts", "acc-9", map[string]any{"object_id": "acc-9", "name": "Compte sans lien"}),
	)
	spec := agentSpec("fill", agentPersonPath)
	spec.Inputs = []core.ObjectRef{{Table: "transactions", ObjectID: "txn-1"}, {Table: "transactions", ObjectID: "txn-2"}}
	return tc, spec
}

func TestPrompt_InlinesInputsAndLinkedParentOnce(t *testing.T) {
	tc, spec := txnAccountContext(t)
	prompt := promptOf(t, tc, spec)

	assert.Contains(t, prompt, "transactions/txn-1 (input) ")
	assert.Contains(t, prompt, `"amount":1200`)
	assert.Contains(t, prompt, `"amount":55`)
	assert.Equal(t, 1, strings.Count(prompt, "accounts/acc-1 (linked)"), "the shared account is written once")
	assert.Contains(t, prompt, "Compte courant ACME")
	assert.NotContains(t, prompt, "Compte sans lien", "an object no input leads to is not written")

	// The block stays untrusted-wrapped.
	start := strings.Index(prompt, "## Selected objects for this task")
	require.GreaterOrEqual(t, start, 0)
	block := prompt[start:]
	open := strings.Index(block, "<data id=")
	closing := strings.Index(block, "</data id=")
	require.True(t, open >= 0 && closing > open)
	assert.Less(t, strings.Index(block, "Compte courant ACME"), closing)
	assert.Greater(t, strings.Index(block, "Compte courant ACME"), open)
}

func TestPrompt_IsDeterministic(t *testing.T) {
	tc, spec := txnAccountContext(t)
	a := nonceRe.ReplaceAllString(promptOf(t, tc, spec), "id=N")
	b := nonceRe.ReplaceAllString(promptOf(t, tc, spec), "id=N")
	assert.Equal(t, a, b)
}

func TestPrompt_PastTheCapObjectsAreOnlyReferenced(t *testing.T) {
	big := strings.Repeat("x", 1500) // under the per-string bound
	var sel []models.SarSelectedObject
	var inputs []core.ObjectRef
	for i := 0; i < 80; i++ {
		id := "t" + string(rune('A'+i/26)) + string(rune('a'+i%26))
		sel = append(sel, agentSelection("transactions", id, map[string]any{"object_id": id, "note": big, "n": i}))
		inputs = append(inputs, core.ObjectRef{Table: "transactions", ObjectID: id})
	}
	tc := agentToolContext(t, sel...)
	spec := agentSpec("fill", agentPersonPath)
	spec.Inputs = inputs
	prompt := promptOf(t, tc, spec)

	assert.Contains(t, prompt, "Not written here")
	inlined := strings.Count(prompt, "(input) ")
	assert.Greater(t, inlined, 0)
	assert.Less(t, inlined, 80)
	assert.LessOrEqual(t, inlined*1500, inlineObjectsMaxBytes, "the inlined data stays under the cap")
	// The last objects are references only, still listed.
	assert.Contains(t, prompt, "transactions/"+inputs[79].ObjectID)
	assert.NotContains(t, prompt, inputs[79].ObjectID+" (input)")
}

func TestPrompt_InputOutsideTheSelectionIsAReferenceOnly(t *testing.T) {
	tc := agentToolContext(t)
	spec := agentSpec("fill", agentPersonPath)
	spec.Inputs = []core.ObjectRef{{Table: "persons", ObjectID: "ghost"}}
	prompt := promptOf(t, tc, spec)
	assert.Contains(t, prompt, "persons/ghost")
	assert.NotContains(t, prompt, "(input)")
}

// ---------- F2 / F3: a prompt that cannot be read ----------

type missingPrompts struct{ fakeAgentPrompts }

func (missingPrompts) Read(path string) (string, error) { return "", errors.New("open " + path + ": no such file") }

func TestAgent_UnreadablePromptIsMarkedMissing(t *testing.T) {
	llm := newScriptedLLM()
	_, err := agRun(t, llm, missingPrompts{}, AgentLimits{}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPromptMissing))
	assert.Equal(t, "prompt manquant : prompts/fill.md", err.Error())
	assert.Equal(t, 0, llm.Calls())
}

func TestRunJob_PromptMissingDuringATaskFailsJobWithoutRetry(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	// The check before the tasks passed, the runner then fails to read it.
	f.Runner.On("a", tgtPP1, Step{Err: promptMissing("prompts/a.md")})

	err := o.RunJob(ctx, f.JobId)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPromptMissing), "the worker cancels the River job on this error")

	j := jobOf(t, f)
	assert.Equal(t, models.SarJobStatusFailed, j.Status)
	assert.Equal(t, "prompt manquant : prompts/a.md", j.Error)
	task := f.Repo.Task(t, "a", tgtPP1)
	assert.Equal(t, models.SarJobStatusFailed, task.Status)
	assert.Equal(t, "prompt manquant : prompts/a.md", task.Error, "the reason is on the task, not only on the job")
}
