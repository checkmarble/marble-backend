package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Fix wave PR 5 (verification-proof items 1-3, verification-jobs item 5).
//
// Some tests set a field that does not exist yet (AgentLimits.CallTimeout,
// TaskSpec.FailJobOnLimit). They go through setField so that this file compiles
// today; each such test fails with "has no field ..." until the field is added.
// Once it exists, the implementer may replace the setField calls by literals.

// agentBirthCountryContext: the default person context, with persons/p1 holding
// a birth_country (the source of paysNaissance).
func agentBirthCountryContext(t *testing.T, country string) core.ToolContext {
	t.Helper()
	return agentToolContext(t,
		agentSelection("persons", "p1", map[string]any{"object_id": "p1", "last_name": "Dupont", "birth_date": "1980-04-01", "birth_country": country}),
	)
}

// agentTransactionContext: one work transaction (ItemSources transactions/t1)
// whose source amount is amount, in EUR.
func agentTransactionContext(t *testing.T, amount any) (core.ToolContext, core.Path) {
	t.Helper()
	tc := agentToolContext(t, agentSelection("transactions", "t1", map[string]any{"object_id": "t1", "amount": amount, "currency": "EUR"}))
	tx := tc.Doc.AddItem("work.transactions", "transactions")
	tc.Doc.ItemSources[tx] = []core.ObjectRef{{Table: "transactions", ObjectID: "t1"}}
	return tc, tx
}

const (
	agPaysNaissancePath = "personnes.pp[pp-1].etatCivil.paysNaissance"
	agPolitiqueePath    = "personnes.pp[pp-1].ppe.politiquementExpose"
	agConnexionPath     = "personnes.pp[pp-1].donneesConnexion"
)

// ---------- A1: copied code-list and boolean fields ----------

// A code whose value matches neither the raw value nor its label, justified but
// without confidence, is sent back: confidence is required for a judgment-like
// correspondence.
func TestAgent_CopiedCodeMismatchWithoutConfidenceIsCorrected(t *testing.T) {
	const noConfidence = `{"path":"` + agPaysNaissancePath + `","value":"FR","source":{"field":"birth_country","raw_value":"Russie"},"justification":"pays cité par le dossier"}`
	const withConfidence = `{"path":"` + agPaysNaissancePath + `","value":"FR","source":{"field":"birth_country","raw_value":"Russie"},"justification":"pays cité par le dossier","confidence":"medium"}`
	llm := newScriptedLLM(FinalStep(agFinal(noConfidence)), FinalStep(agFinal(withConfidence)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentBirthCountryContext(t, "Russie"), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	require.Equal(t, 2, llm.Calls(), "the answer without confidence is sent back once")
	agLastIsCorrection(t, llm.Requests[1])
	require.Len(t, out.Fills, 1)
	assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence, "the engine forces low on an exported judgment-like copy")
}

// A literal match is accepted without justification nor confidence (guard: it
// already holds today).
func TestAgent_CopiedCodeLiteralMatchAccepted(t *testing.T) {
	const literal = `{"path":"` + agPaysNaissancePath + `","value":"RU","source":{"field":"birth_country","raw_value":"RU"}}`
	llm := newScriptedLLM(FinalStep(agFinal(literal)), FinalStep(agFinal(literal)), FinalStep(agFinal(literal)), FinalStep(agFinal(literal)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentBirthCountryContext(t, "RU"), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	assert.Equal(t, 1, llm.Calls(), "a literal match is not sent back")
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "RU", out.Fills[0].Value)
}

// A label match (code RU, label « Russie » in liste_ref_pays.csv) is a mechanical
// conversion: accepted without justification nor confidence.
func TestAgent_CopiedCodeLabelMatchAccepted(t *testing.T) {
	const byLabel = `{"path":"` + agPaysNaissancePath + `","value":"RU","source":{"field":"birth_country","raw_value":"Russie"}}`
	llm := newScriptedLLM(FinalStep(agFinal(byLabel)), FinalStep(agFinal(byLabel)), FinalStep(agFinal(byLabel)), FinalStep(agFinal(byLabel)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentBirthCountryContext(t, "Russie"), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	assert.Equal(t, 1, llm.Calls(), "a label match is not sent back")
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "RU", out.Fills[0].Value)
}

// A code that mismatches its source, with justification and confidence, is
// accepted, and the engine forces the confidence to low whatever the model said.
func TestAgent_CopiedCodeMismatchWithConfidenceAcceptedForcedLow(t *testing.T) {
	const fill = `{"path":"` + agPolitiqueePath + `","value":"Non","source":{"field":"last_name","raw_value":"Dupont"},"justification":"aucune mention dans le dossier","confidence":"high"}`
	llm := newScriptedLLM(FinalStep(agFinal(fill)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	assert.Equal(t, 1, llm.Calls())
	require.Len(t, out.Fills, 1)
	assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence)
	assert.Equal(t, "aucune mention dans le dossier", out.Fills[0].Justification)
}

// A copied boolean whose source is not a boolean literal: justification alone is
// not enough (sent back); with a confidence it is accepted and forced to low.
func TestAgent_CopiedBooleanNeedsConfidenceAndIsForcedLow(t *testing.T) {
	const noConfidence = `{"path":"` + agConnexionPath + `","value":"true","source":{"field":"last_name","raw_value":"Dupont"},"justification":"déduit du nom"}`
	const withConfidence = `{"path":"` + agConnexionPath + `","value":"true","source":{"field":"last_name","raw_value":"Dupont"},"justification":"déduit du nom","confidence":"high"}`
	llm := newScriptedLLM(FinalStep(agFinal(noConfidence)), FinalStep(agFinal(withConfidence)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	require.Equal(t, 2, llm.Calls(), "the boolean without confidence is sent back once")
	agLastIsCorrection(t, llm.Requests[1])
	require.Len(t, out.Fills, 1)
	assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence)
}

// ---------- A2: per-call timeout and budget checked before each turn ----------

// blockingLLM never answers: it waits for its context. It records whether the
// context ended by its deadline; a 10 s safety net keeps a missing timeout from
// hanging the suite.
type blockingLLM struct {
	mu          sync.Mutex
	calls       int
	deadlineHit bool
}

func (b *blockingLLM) Step(ctx context.Context, _ AgentRequest) (AgentStep, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	select {
	case <-ctx.Done():
		b.mu.Lock()
		b.deadlineHit = errors.Is(ctx.Err(), context.DeadlineExceeded)
		b.mu.Unlock()
		return AgentStep{}, ctx.Err()
	case <-time.After(10 * time.Second):
		return AgentStep{}, errors.New("safety net: the per-call timeout never fired")
	}
}

// slowLLM sleeps before answering, ignoring the context, like a provider that
// does not honour cancellation.
type slowLLM struct {
	inner LLM
	delay time.Duration
}

func (s slowLLM) Step(ctx context.Context, req AgentRequest) (AgentStep, error) {
	time.Sleep(s.delay)
	return s.inner.Step(ctx, req)
}

// A provider call that outlives its own per-call timeout is a technical error:
// the task fails (River retries it) and is not done with an empty output.
func TestAgent_ProviderCallPastItsTimeoutIsTechnicalError(t *testing.T) {
	b := &blockingLLM{}
	limits := AgentLimits{Timeout: time.Minute, CallTimeout: 50 * time.Millisecond}
	out, err := agRun(t, b, fakeAgentPrompts{}, limits, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.True(t, b.deadlineHit, "the call was cut by its own per-call deadline (calls=%d)", b.calls)
	require.Error(t, err, "a call past its per-call timeout is a technical error, not a quality limit")
	assert.Empty(t, out.Fills)
	assert.Empty(t, out.Questions, "no limit output: the task is not done")
}

// The task time budget is checked before each turn: once it is exhausted, no
// further LLM call is made and the run ends on the limit path.
func TestAgent_BudgetCheckedBeforeEachTurn(t *testing.T) {
	scripted := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		FinalStep(agFinal(agNomFillValid)),
	)
	// The first call takes longer than the whole task budget.
	llm := slowLLM{inner: scripted, delay: 100 * time.Millisecond}
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{Timeout: 30 * time.Millisecond}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err, "a budget exhausted before a turn is a quality limit")
	assert.Equal(t, 1, scripted.Calls(), "no LLM call once the budget is exhausted")
	assert.Empty(t, out.Fills)
	assert.NotEmpty(t, out.Questions, "the unfilled required paths become questions")
}

// ---------- A3: decimal value in cents ----------

// raw 123456 (cents) -> 1234.56 (euros) with a justification is accepted, with
// the confidence forced to low.
func TestAgent_DecimalCentsConvertedWithJustificationAcceptedForcedLow(t *testing.T) {
	tc, tx := agentTransactionContext(t, 123456)
	fill := `{"path":"` + string(tx) + `.montant","value":"1234.56","source":{"field":"amount","raw_value":"123456"},"justification":"montant en centimes, divisé par 100","confidence":"high"}`
	llm := newScriptedLLM(FinalStep(agFinal(fill)), FinalStep(agFinal(fill)), FinalStep(agFinal(fill)), FinalStep(agFinal(fill)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, tc, agentSpec("fill_transaction", tx))
	require.NoError(t, err)
	assert.Equal(t, 1, llm.Calls(), "a cents conversion with justification is not sent back")
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "1234.56", out.Fills[0].Value)
	assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence)
}

// The same conversion without justification stays refused (the rule is the
// justification plus the forced low confidence).
func TestAgent_DecimalCentsWithoutJustificationRefused(t *testing.T) {
	tc, tx := agentTransactionContext(t, 123456)
	fill := `{"path":"` + string(tx) + `.montant","value":"1234.56","source":{"field":"amount","raw_value":"123456"}}`
	llm := newScriptedLLM(FinalStep(agFinal(fill)), FinalStep(agFinal(fill)), FinalStep(agFinal(fill)), FinalStep(agFinal(fill)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, tc, agentSpec("fill_transaction", tx))
	require.NoError(t, err)
	assert.Empty(t, out.Fills)
	assert.NotEmpty(t, out.Questions)
}

// 12345.6 for raw 123456 is not the cents conversion (x100 is 1234560): refused
// even with a justification.
func TestAgent_DecimalWrongScaleRefusedEvenWithJustification(t *testing.T) {
	tc, tx := agentTransactionContext(t, 123456)
	fill := `{"path":"` + string(tx) + `.montant","value":"12345.6","source":{"field":"amount","raw_value":"123456"},"justification":"montant en centimes, divisé par 100","confidence":"low"}`
	llm := newScriptedLLM(FinalStep(agFinal(fill)), FinalStep(agFinal(fill)), FinalStep(agFinal(fill)), FinalStep(agFinal(fill)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, tc, agentSpec("fill_transaction", tx))
	require.NoError(t, err)
	assert.Empty(t, out.Fills, "the wrong scale is never applied")
	assert.NotEmpty(t, out.Questions)
}

// ---------- A4 (agent level): FailJobOnLimit ----------

// agentPlaceSpec is a structured task (no fills), like the TRACFIN placement.
func agentPlaceSpec() core.TaskSpec {
	return core.TaskSpec{
		Kind: "place", PromptPath: "prompts/place.md", Tier: core.TierLight,
		ToolNames:    []string{"read_object"},
		OutputSchema: &jsonschema.Schema{Type: "object"},
	}
}

// A flagged task that ends on the turn limit with no structured output is an
// error (the job must fail), not a done task with an empty output.
func TestAgent_FailJobOnLimitWithoutOutputIsAnError(t *testing.T) {
	spec := agentPlaceSpec()
	spec.FailJobOnLimit = true
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
	)
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 1}, agentToolContext(t), spec)
	require.Error(t, err, "a flagged task ending on a limit with no usable output fails")
	assert.Empty(t, out.Structured)
}

// Control: the same limit on an unflagged task stays a quality limit (done, with
// no structured output).
func TestAgent_LimitWithoutFailJobOnLimitIsNotAnError(t *testing.T) {
	spec := agentPlaceSpec()
	llm := newScriptedLLM(
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
		ToolCallStep("read_object", agReadP1Args),
	)
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxTurns: 1}, agentToolContext(t), spec)
	require.NoError(t, err)
	assert.Empty(t, out.Structured)
}
