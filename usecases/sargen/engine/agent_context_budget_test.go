package engine

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/checkmarble/llmberjack"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// ---------- D: a prepared answer is never lost ----------

const agValidateNomArgs = `{"fills":[{"path":"` + agNomPath + `","value":"Dupont","source":{"field":"last_name","raw_value":"Dupont"}}]}`

func TestAgent_GraceTurnKeepsToolsAndRefusesTheirCalls(t *testing.T) {
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args), // grace turn: the model calls a tool anyway
	)
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 1}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)

	require.Equal(t, 2, llm.Calls())
	assert.NotEmpty(t, llm.Requests[1].Tools, "tool declarations stay: some providers reject a tool history without them")
	assert.Equal(t, agToolNames(llm.Requests[0].Tools), agToolNames(llm.Requests[1].Tools))
	last := llm.Requests[1].History
	assert.Equal(t, budgetMessage, last[len(last)-1].Text)
	require.NotEmpty(t, out.Questions)
	assert.Contains(t, out.Questions[0].Text, "à renseigner manuellement", "the refused call falls to the limit output")
}

func TestAgent_ProviderErrorOnTheGraceTurnEndsWithTheLimitOutput(t *testing.T) {
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ErrStep(errors.New("provider rejected the request")),
	)
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 1}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err, "never a technical failure once only the answer was asked")
	require.NotEmpty(t, out.Questions)
	assert.Contains(t, out.Questions[0].Text, "à renseigner manuellement")
}

func TestAgent_ProviderErrorBeforeTheBudgetIsStillTechnical(t *testing.T) {
	llm := newScriptedLLM(ErrStep(errors.New("provider down")))
	_, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 5}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.Error(t, err)
}

func TestAgent_ValidateWithFillsOnTheLastStepKeepsItsProof(t *testing.T) {
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("validate", agValidateNomArgs), // the model ignores that no more tools are run
	)
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 1}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)

	assert.Equal(t, 2, llm.Calls(), "no further turn after the validate call")
	require.Len(t, out.Fills, 1, "a copied value with its source is applied")
	assert.Equal(t, core.Path(agNomPath), out.Fills[0].Path)
	assert.Equal(t, "Dupont", out.Fills[0].Value)
	require.Len(t, out.Fills[0].Sources, 1)
	// The required paths it does not cover get the limit question, like with no answer.
	var manual bool
	for _, q := range out.Questions {
		manual = manual || strings.Contains(q.Text, "à renseigner manuellement")
	}
	assert.True(t, manual, "%v", out.Questions)
}

func TestAgent_ValidateRecoveryMergesWhatWasAlreadyValidated(t *testing.T) {
	// First final: one valid fill and one invalid one (sent back for correction).
	first := agFinal(agNomFillValid,
		`{"path":"personnes.pp[pp-1].etatCivil.prenom","value":"X","source":{"field":"last_name","raw_value":"Nope"}}`)
	llm := newScriptedLLM(
		FinalStep(first),
		ToolCallStep("validate", `{"fills":[{"path":"personnes.pp[pp-1].etatCivil.prenom","value":"Y"}]}`),
		ToolCallStep("validate", agValidateNomArgs),
	)
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 2}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)

	var paths []core.Path
	for _, f := range out.Fills {
		paths = append(paths, f.Path)
	}
	assert.Contains(t, paths, core.Path(agNomPath), "the earlier validated fill survives")
	assert.Len(t, paths, 1, "no duplicate of a path")
}

func TestAgent_ValidateToolReportsWhatTheFinalAnswerWouldRefuse(t *testing.T) {
	llm := newScriptedLLM(
		ToolCallStep("validate", `{"fills":[{"path":"`+agNomPath+`","value":"Dupont"}]}`), // no source
		FinalStep(agFinal(agNomFillValid)),
	)
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	require.Len(t, out.Fills, 1)
	hist := llm.Requests[1].History
	toolMsg := hist[len(hist)-1].Text
	assert.Contains(t, toolMsg, "source", "the dry run says a copied value needs a source: %s", toolMsg)
	assert.Contains(t, toolMsg, `"accepted":0`)
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

func TestAgent_TimeLeftUnderOneCallGrantsTheGraceTurn(t *testing.T) {
	slow := &delayLLM{delay: 30 * time.Millisecond, inner: newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		FinalStep(agFinal(agNomFillValid)),
	)}
	out, err := agRun(t, slow, fakeAgentPrompts{}, AgentLimits{MaxTurns: 10, Timeout: 50 * time.Millisecond},
		agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	assert.Equal(t, 2, slow.inner.Calls(), "a grace turn was granted instead of returning the limit output")
	require.Len(t, out.Fills, 1, "the answer given on the grace turn is kept")
}

// delayLLM delays every call.
type delayLLM struct {
	delay time.Duration
	inner *scriptedLLM
}

func (l *delayLLM) Step(ctx context.Context, req AgentRequest) (AgentStep, error) {
	time.Sleep(l.delay)
	return l.inner.Step(ctx, req)
}

func TestBuildTurnRequest_GraceTurnKeepsTheToolDeclarations(t *testing.T) {
	thread := &llmberjack.ThreadId{}
	st := turnTwoRun(thread, llmberjack.ResponseToolCall{Id: "1", Name: "read_object", Parameters: []byte(`{"table":"persons","object_id":"p1"}`)})
	c1 := ToolCall{ID: "1", Name: "read_object"}
	req := turnTwoRequest(t, AgentMessage{Role: RoleTool, Text: "result", ToolCall: &c1},
		AgentMessage{Role: RoleUser, Text: budgetMessage})
	r, err := buildTurnRequest(st, req, "main", "m")
	require.NoError(t, err)
	assert.NotEmpty(t, r.Tools)
	require.Len(t, r.Messages, 2)
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
	tc.Doc.ItemSources[agentPersonPath] = spec.Inputs // what the task may cite: its item sources and one hop
	return tc, spec
}

// Only what the task may cite is written: the holder of the account is two hops
// from the item's sources, it is not shown as if it were citable.
func TestPrompt_OnlyCitableLinkedObjectsAreWritten(t *testing.T) {
	tc, spec := txnAccountContext(t)
	acc := tc.DataModel.Tables["accounts"]
	acc.Fields["holder_id"] = models.Field{Name: "holder_id", DataType: models.String}
	acc.LinksToSingle = map[string]models.LinkToSingle{"holder": {Name: "holder", ChildTableName: "accounts",
		ChildFieldName: "holder_id", ParentTableName: "holders", ParentFieldName: "object_id"}}
	tc.DataModel.Tables["accounts"] = acc
	tc.DataModel.Tables["holders"] = models.Table{Name: "holders", Fields: map[string]models.Field{"object_id": {Name: "object_id", DataType: models.String}}}
	for i := range tc.Selection {
		if tc.Selection[i].ObjectId == "acc-1" {
			tc.Selection[i].Snapshot["holder_id"] = "h-1"
		}
	}
	tc.Selection = append(tc.Selection, agentSelection("holders", "h-1", map[string]any{"object_id": "h-1", "name": "Titulaire Deep"}))
	prompt := promptOf(t, tc, spec)
	assert.Contains(t, prompt, "Compte courant ACME")
	assert.NotContains(t, prompt, "Titulaire Deep")
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

func (missingPrompts) Read(path string) (string, error) {
	return "", errors.New("open " + path + ": no such file")
}

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
	f.Runner.On("a", tgtPP1, Step{Err: promptMissing("prompts/a.md", errors.New("open: no such file"))})

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

func TestPrompt_AutoAddedObjectsAreMarkedAsContext(t *testing.T) {
	tc, spec := txnAccountContext(t)
	for i := range tc.Selection {
		if tc.Selection[i].TableName == "accounts" && tc.Selection[i].ObjectId == "acc-1" {
			tc.Selection[i].AutoAdded, tc.Selection[i].ContextOnly = true, true
		}
	}
	prompt := promptOf(t, tc, spec)
	assert.Contains(t, prompt, "accounts/acc-1 (linked, context-only)")
	legend := strings.Index(prompt, contextOnlyLegend)
	block := strings.Index(prompt, "selected_objects")
	require.GreaterOrEqual(t, legend, 0, "the legend is written")
	assert.Less(t, legend, block, "the legend sits outside the untrusted block")
	assert.NotContains(t, prompt[block:], "may be cited", "no guidance inside the untrusted block")
	assert.Contains(t, prompt, "transactions/txn-1 (input) ")
}

func TestSystemInstructionAsksForParallelToolCalls(t *testing.T) {
	assert.Contains(t, systemInstruction, "request them all in the same turn")
}

// A missing prompt wins over another task's error in the same wave: the worker
// must cancel (no retry) even when the other failure came first.
func TestRunJob_MissingPromptWinsOverAnotherTaskError(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	a, b := Spec("a", tgtPP1), Spec("b", tgtPP2)
	f.Format.Waves = [][]core.TaskSpec{{a, b}}
	f.Runner.On("a", tgtPP1, Step{Err: errProvider})
	f.Runner.On("b", tgtPP2, Step{Err: promptMissing("prompts/b.md", nil), Before: func() { time.Sleep(40 * time.Millisecond) }})

	err := o.RunJob(ctx, f.JobId)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPromptMissing), "%v", err)
}

// A rate limit on the grace turn keeps its job-level handling: it propagates
// (the job snoozes and the task is redone later with its whole budget).
func TestAgent_RateLimitOnTheGraceTurnPropagates(t *testing.T) {
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ErrStep(errors.Wrap(models.LLMRateLimitedError, "quota")),
	)
	_, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 1}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.Error(t, err)
	assert.True(t, errors.Is(err, models.LLMRateLimitedError))
}

func TestAgent_UnavailableModelOnTheGraceTurnPropagates(t *testing.T) {
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ErrStep(unavailable(errors.New("aucun modèle"))),
	)
	_, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 1}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLLMUnavailable))
}

// After a recovery, a path that got a « valeur non vérifiable » question is not
// asked about again by the limit question.
func TestAgent_RecoveryAsksOnceAboutAPath(t *testing.T) {
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		// copied value without source: becomes « valeur non vérifiable » on this path
		ToolCallStep("validate", `{"fills":[{"path":"personnes.pp[pp-1].implication","value":"Je ne sais pas"}]}`),
	)
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 1}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	count := map[core.Path]int{}
	for _, q := range out.Questions {
		for _, p := range q.Paths {
			count[p]++
		}
	}
	for p, n := range count {
		assert.Equal(t, 1, n, "one question per path: %s", p)
	}
	assert.Equal(t, 1, count["personnes.pp[pp-1].implication"])
	var unverifiable bool
	for _, q := range out.Questions {
		for _, p := range q.Paths {
			if p == "personnes.pp[pp-1].implication" {
				unverifiable = strings.Contains(q.Text, "valeur non vérifiable")
			}
		}
	}
	assert.True(t, unverifiable, "the « valeur non vérifiable » question wins: %v", out.Questions)
}
