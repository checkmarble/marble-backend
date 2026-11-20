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
	other := `{"path":"personnes.pm[pm-1].activiteCoordonnee.enseigneSigle","value":"Dupont","source":{"field":"last_name","raw_value":"Dupont"}}`
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

// A rule raised on a group (the other address is forbidden outside SIREN) is
// matched to the fill written under that group.
func TestCheckFills_GroupRuleIssueGoesBackForTheFillUnderTheGroup(t *testing.T) {
	c := e2Checker(t, "RN6", false)
	city := "personnes.pm[pm-1].activiteCoordonnee.autreAdresse.ville"
	fill := `{"path":"` + city + `","value":"Dupont","source":{"field":"last_name","raw_value":"Dupont"}}`
	res := c.checkFills(json.RawMessage(`{"fills":[` + fill + `]}`))
	assert.Empty(t, res.Fills)
	require.Len(t, res.Corrections, 1, "%v", res.Corrections)
	assert.Contains(t, res.Corrections[0], "fills[0] "+city)
	assert.Contains(t, res.Corrections[0], "SIREN")
}

// A scope-level exclusivity rule (birth date and estimated date: one or the
// other) is matched to the fill written under the scope.
func TestCheckFills_ScopeLevelRuleGoesBackForTheFill(t *testing.T) {
	tc := agentToolContext(t)
	scope := "personnes.pp[pp-1].etatCivil"
	tc.Doc.Values[core.Path(scope+".dateNaissance")] = core.Value{Value: "1980-04-01", Origin: core.OriginLLM}
	spec := agentSpec("fill", agentPersonPath)
	c := newChecker(tc, spec, tc.TaskId, []core.Path{agentPersonPath})
	fill := `{"path":"` + scope + `.dateNaissanceEstimation","value":"1980","source":{"field":"birth_date","raw_value":"1980-04-01"}}`
	res := c.checkFills(json.RawMessage(`{"fills":[` + fill + `]}`))
	assert.Empty(t, res.Fills, "%v", res.Corrections)
	require.Len(t, res.Corrections, 1)
	assert.Contains(t, res.Corrections[0], "l'une ou l'autre")
}

// Once the correction budget is spent on a warning-level rule, the fill is
// dropped silently: no question.
func TestAgent_WarningRuleAfterTheCorrectionBudgetDropsTheFillSilently(t *testing.T) {
	c := e2Checker(t, "SIREN", false)
	final := agFinal(e2FillJSON)
	llm := newScriptedLLM(FinalStep(final), FinalStep(final))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxCorrections: 1}, c.tc, c.spec)
	require.NoError(t, err)
	assert.Equal(t, 2, llm.Calls(), "one correction turn")
	assert.Empty(t, out.Fills)
	for _, q := range out.Questions {
		assert.NotContains(t, q.Text, "valeur non vérifiable")
	}
}

// A scope-level issue is blamed on the fill that causes it, not on the sibling
// fills under the same scope.
func TestCheckFills_ScopeLevelRuleBlamesOnlyTheCulpritFill(t *testing.T) {
	tc := agentToolContext(t)
	scope := "personnes.pp[pp-1].etatCivil"
	tc.Doc.Values[core.Path(scope+".dateNaissance")] = core.Value{Value: "1980-04-01", Origin: core.OriginLLM}
	c := newChecker(tc, agentSpec("fill", agentPersonPath), tc.TaskId, []core.Path{agentPersonPath})
	estimation := `{"path":"` + scope + `.dateNaissanceEstimation","value":"1980","source":{"field":"birth_date","raw_value":"1980-04-01"}}`
	res := c.checkFills(json.RawMessage(`{"fills":[` + estimation + `,` + agNomFillValid + `]}`))
	require.Len(t, res.Corrections, 1, "%v", res.Corrections)
	assert.Contains(t, res.Corrections[0], "dateNaissanceEstimation")
	require.Len(t, res.Fills, 1)
	assert.Equal(t, core.Path(agNomPath), res.Fills[0].Path)
}

// A group rule that two fills trigger together (each alone leaves the group
// present) is blamed on both.
func TestCheckFills_GroupRuleBlamesAllFillsWhenNoneIsAloneToBlame(t *testing.T) {
	c := e2Checker(t, "RN6", false)
	g := "personnes.pm[pm-1].activiteCoordonnee.autreAdresse."
	f1 := `{"path":"` + g + `ville","value":"Dupont","source":{"field":"last_name","raw_value":"Dupont"}}`
	f2 := `{"path":"` + g + `nomVoie","value":"Dupont","source":{"field":"last_name","raw_value":"Dupont"}}`
	res := c.checkFills(json.RawMessage(`{"fills":[` + f1 + `,` + f2 + `]}`))
	assert.Empty(t, res.Fills)
	assert.Len(t, res.Corrections, 2, "%v", res.Corrections)
}

// A group rule with a single fill under the group blames that fill, and the
// sibling fill elsewhere in the item is kept.
func TestCheckFills_GroupRuleKeepsTheSiblingOutsideTheGroup(t *testing.T) {
	c := e2Checker(t, "RN6", false)
	city := `{"path":"personnes.pm[pm-1].activiteCoordonnee.autreAdresse.ville","value":"Dupont","source":{"field":"last_name","raw_value":"Dupont"}}`
	denom := `{"path":"personnes.pm[pm-1].activiteCoordonnee.enseigneSigle","value":"Dupont","source":{"field":"last_name","raw_value":"Dupont"}}`
	res := c.checkFills(json.RawMessage(`{"fills":[` + city + `,` + denom + `]}`))
	require.Len(t, res.Corrections, 1, "%v", res.Corrections)
	assert.Contains(t, res.Corrections[0], "autreAdresse.ville")
	require.Len(t, res.Fills, 1)
}

func TestHardErrors_DropsWarningLevelErrors(t *testing.T) {
	a := &agentRun{soft: map[string]bool{"fills[0] p : w": true}}
	assert.Equal(t, []string{"fills[1] q : b"}, a.hardErrors([]string{"fills[0] p : w", "fills[1] q : b"}))
}
