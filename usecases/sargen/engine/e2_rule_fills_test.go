package engine

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const (
	e2Item      = core.Path("personnes.pm[pm-1]")
	e2RegType   = "personnes.pm[pm-1].activiteCoordonnee.numeroIdentification.type"
	e2Enseigne  = "personnes.pm[pm-1].activiteCoordonnee.enseigneSigle"
	e2FillJSON  = `{"path":"` + e2Enseigne + `","value":"Dupont","source":{"field":"last_name","raw_value":"Dupont"}}`
	e2RuleMsgIn = "SIREN"
)

// e2Checker is a checker on a legal entity item whose registry type is registryType;
// an enseigneSigle value may already be present (an issue the task did not cause).
func e2Checker(t *testing.T, registryType string, existingEnseigne bool) *checker {
	t.Helper()
	tc := agentToolContext(t)
	pm := tc.Doc.AddItem("personnes.pm", "pm")
	require.Equal(t, e2Item, pm)
	tc.Doc.ItemSources[pm] = []core.ObjectRef{{Table: "persons", ObjectID: "p1"}}
	tc.Doc.Values[core.Path(e2RegType)] = core.Value{Value: registryType, Origin: core.OriginLLM}
	if existingEnseigne {
		tc.Doc.Values[core.Path(e2Enseigne)] = core.Value{Value: "Ancien", Origin: core.OriginLLM}
	}
	spec := agentSpec("fill", e2Item)
	spec.Inputs = []core.ObjectRef{{Table: "persons", ObjectID: "p1"}}
	return newChecker(tc, spec, tc.TaskId, []core.Path{e2Item})
}

// A fill that triggers a format rule (a warning too) is sent back, with the
// message of the rule.
func TestCheckFills_RuleIssueOfTheFillGoesBackForCorrection(t *testing.T) {
	c := e2Checker(t, "SIREN", false)
	res := c.checkFills(json.RawMessage(`{"fills":[` + e2FillJSON + `]}`))
	assert.Empty(t, res.Fills)
	require.Len(t, res.Corrections, 1)
	assert.Contains(t, res.Corrections[0], "fills[0] "+e2Enseigne)
	assert.Contains(t, res.Corrections[0], e2RuleMsgIn)
}

func TestCheckFills_SameFillPassesWhenTheRegistryIsNotSiren(t *testing.T) {
	c := e2Checker(t, "RN6", false)
	res := c.checkFills(json.RawMessage(`{"fills":[` + e2FillJSON + `]}`))
	assert.Empty(t, res.Corrections)
	require.Len(t, res.Fills, 1)
}

// A rule issue that was already in the document, on a path this task writes
// again, is not new: the fill stays (nothing is made worse by it).
func TestCheckFills_ExistingRuleIssueDoesNotBlockTheFills(t *testing.T) {
	c := e2Checker(t, "SIREN", true)
	res := c.checkFills(json.RawMessage(`{"fills":[` + e2FillJSON + `]}`))
	assert.Empty(t, res.Corrections)
	require.Len(t, res.Fills, 1)
}

// An existing rule issue on a path the task does not write does not block the
// task's other fills.
func TestCheckFills_RuleIssueOnAnotherPathIsIgnored(t *testing.T) {
	c := e2Checker(t, "SIREN", true)
	other := `{"path":"personnes.pm[pm-1].activiteCoordonnee.denomination","value":"Dupont","source":{"field":"last_name","raw_value":"Dupont"}}`
	res := c.checkFills(json.RawMessage(`{"fills":[` + other + `]}`))
	for _, m := range res.Corrections {
		assert.NotContains(t, m, e2Enseigne)
	}
}

func TestDryRun_ReportsTheRuleIssue(t *testing.T) {
	c := e2Checker(t, "SIREN", false)
	out := c.dryRun(json.RawMessage(`{"fills":[` + e2FillJSON + `]}`))
	assert.Contains(t, out, e2RuleMsgIn)
	assert.Contains(t, out, `"accepted":0`)
}

// Once the correction budget is spent, the fill that still triggers the rule is
// not applied and becomes a « valeur non vérifiable » question, like any fill
// that stays invalid.
func TestAgent_RuleIssueAfterTheCorrectionBudgetBecomesAQuestion(t *testing.T) {
	c := e2Checker(t, "SIREN", false)
	final := agFinal(e2FillJSON)
	llm := newScriptedLLM(FinalStep(final), FinalStep(final))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxCorrections: 1}, c.tc, c.spec)
	require.NoError(t, err)
	assert.Equal(t, 2, llm.Calls(), "one correction turn")
	assert.Empty(t, out.Fills)
	require.Len(t, out.Questions, 1)
	assert.Equal(t, []core.Path{e2Enseigne}, out.Questions[0].Paths)
	assert.Contains(t, out.Questions[0].Text, "valeur non vérifiable")
}
