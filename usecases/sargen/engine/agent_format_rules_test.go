package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Format rules wave: unknown code of a code list (A2), format instructions
// prepended by the engine (B1), French reasons for the analyst (C).

const agPEPContextField = "is_pep"

func agentPEPContext(t *testing.T, snapshot map[string]any) core.ToolContext {
	t.Helper()
	base := map[string]any{"object_id": "p1", "last_name": "Dupont", "kyc_risk": "low"}
	for k, v := range snapshot {
		base[k] = v
	}
	tc := agentToolContext(t, agentSelection("persons", "p1", base),
		agentSelection("persons", "p2", map[string]any{"object_id": "p2", "last_name": "Martin"}))
	tc.DataModel.Tables["persons"] = models.Table{Name: "persons", Fields: map[string]models.Field{
		"object_id": {Name: "object_id", DataType: models.String},
		"is_pep":    {Name: "is_pep", DataType: models.Bool},
		"nb_flags":  {Name: "nb_flags", DataType: models.Int},
		"siren":     {Name: "siren", DataType: models.String},
	}}
	return tc
}

func pepFill(value, field, raw, extra string) string {
	return `{"path":"` + agPolitiqueePath + `","value":"` + value + `","source":{"field":"` + field + `","raw_value":"` + raw + `"}` + extra + `}`
}

// « Non » from a KYC risk level « low » (a weak relation, justified) is not
// applied: it becomes a question whose options are the codes of the list, each
// with its fill.
func TestAgent_UnknownCodeList_WeakProofBecomesQuestion(t *testing.T) {
	fill := pepFill("Non", "kyc_risk", "low", `,"justification":"risque faible","confidence":"medium"`)
	llm := newScriptedLLM(FinalStep(agFinal(fill)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, nil), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	assert.Equal(t, 1, llm.Calls(), "not sent back to the model: the question is the outcome")
	assert.Empty(t, out.Fills)
	require.Len(t, out.Questions, 1)
	q := out.Questions[0]
	assert.Equal(t, []core.Path{agPolitiqueePath}, q.Paths)
	require.Len(t, q.Options, 3)
	var labels []string
	for _, o := range q.Options {
		labels = append(labels, o.Label)
		require.Len(t, o.Fills, 1, "each option carries its fill")
		assert.Equal(t, core.Path(agPolitiqueePath), o.Fills[0].Path)
		assert.Equal(t, core.OriginAnalyst, o.Fills[0].Origin)
	}
	assert.Equal(t, []string{"Oui", "Non", "Je ne sais pas"}, labels)
	assert.Equal(t, "Non", q.Options[1].Fills[0].Value)
}

// No source at all, for another code than the unknown one: same question.
func TestAgent_UnknownCodeList_NoSourceBecomesQuestion(t *testing.T) {
	fill := `{"path":"` + agPolitiqueePath + `","value":"Non"}`
	llm := newScriptedLLM(FinalStep(agFinal(fill)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, nil), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	assert.Equal(t, 1, llm.Calls())
	assert.Empty(t, out.Fills)
	require.Len(t, out.Questions, 1)
	assert.Len(t, out.Questions[0].Options, 3)
}

// The unknown code itself is always accepted, without source.
func TestAgent_UnknownCodeList_UnknownCodeAcceptedWithoutSource(t *testing.T) {
	fill := `{"path":"` + agPolitiqueePath + `","value":"Je ne sais pas"}`
	llm := newScriptedLLM(FinalStep(agFinal(fill)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, nil), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	assert.Equal(t, 1, llm.Calls())
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "Je ne sais pas", out.Fills[0].Value)
	assert.Empty(t, out.Questions)
}

// A boolean source is a strict proof for a yes/no list; so is the code itself or
// its label.
func TestAgent_UnknownCodeList_StrictProofAccepted(t *testing.T) {
	t.Run("boolean source", func(t *testing.T) {
		llm := newScriptedLLM(FinalStep(agFinal(pepFill("Oui", agPEPContextField, "true", ""))))
		out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, map[string]any{agPEPContextField: true}), agentSpec("fill", agentPersonPath))
		require.NoError(t, err)
		assert.Equal(t, 1, llm.Calls())
		require.Len(t, out.Fills, 1)
		assert.Equal(t, "Oui", out.Fills[0].Value)
		assert.Empty(t, out.Questions)
	})
	t.Run("boolean source disagreeing", func(t *testing.T) {
		llm := newScriptedLLM(FinalStep(agFinal(pepFill("Non", agPEPContextField, "true", ""))))
		out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, map[string]any{agPEPContextField: true}), agentSpec("fill", agentPersonPath))
		require.NoError(t, err)
		assert.Empty(t, out.Fills)
		require.Len(t, out.Questions, 1)
	})
	t.Run("raw value is the code", func(t *testing.T) {
		llm := newScriptedLLM(FinalStep(agFinal(pepFill("Non", "pep_status", "Non", ""))))
		out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, map[string]any{"pep_status": "Non"}), agentSpec("fill", agentPersonPath))
		require.NoError(t, err)
		require.Len(t, out.Fills, 1)
		assert.Empty(t, out.Questions)
	})
}

// A list without unknown code keeps its behavior: a justified mismatch is
// accepted (forced low), see TestAgent_CopiedCodeMismatchWithConfidenceAcceptedForcedLow.
// The option fills of a question stay the analyst's choice, with no proof.
func TestAgent_UnknownCodeList_OptionFillsNeedNoProof(t *testing.T) {
	final := `{"fills":[],"questions":[{"paths":["` + agPolitiqueePath + `"],"text":"PPE ?","options":[` +
		`{"label":"Non","fills":[{"path":"` + agPolitiqueePath + `","value":"Non"}]}]}]}`
	llm := newScriptedLLM(FinalStep(final))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, nil), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	require.Len(t, out.Questions, 1)
	require.Len(t, out.Questions[0].Options[0].Fills, 1)
}

// A boolean literal proves a yes/no code only when the source field is a boolean
// in the data model: a count of 1 or a score of 0 prove nothing.
func TestAgent_BooleanLiteralNeedsBooleanField(t *testing.T) {
	llm := newScriptedLLM(FinalStep(agFinal(pepFill("Oui", "nb_flags", "1", `,"justification":"un signalement","confidence":"medium"`))))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, map[string]any{"nb_flags": 1}), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	require.Len(t, out.Fills, 1, "Oui is not a strict code: a weak relation with a justification is accepted")
	assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence)

	llm = newScriptedLLM(FinalStep(agFinal(pepFill("Non", "nb_flags", "0", ""))))
	out, err = agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, map[string]any{"nb_flags": 0}), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	assert.Empty(t, out.Fills, "Non from a numeric 0 is not a strict proof")
	require.Len(t, out.Questions, 1)
}

// Oui is not a strict code: a weak source with a justification is accepted, low.
func TestAgent_NonStrictCodeWeakSourceAccepted(t *testing.T) {
	fill := pepFill("Oui", "kyc_risk", "low", `,"justification":"surveillance renforcée","confidence":"medium"`)
	llm := newScriptedLLM(FinalStep(agFinal(fill)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, nil), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "Oui", out.Fills[0].Value)
	assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence)
	assert.Empty(t, out.Questions)
}

// The unknown code with an unrelated source is accepted and the source dropped.
func TestAgent_UnknownCodeUnrelatedSourceDropped(t *testing.T) {
	fill := pepFill("Je ne sais pas", "kyc_risk", "low", "")
	llm := newScriptedLLM(FinalStep(agFinal(fill)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, nil), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	assert.Equal(t, 1, llm.Calls(), "no correction round")
	require.Len(t, out.Fills, 1)
	assert.Empty(t, out.Fills[0].Sources)
}

// Registry type: a SIREN number cited from a siren column, with a justification,
// is accepted (low); Non is strict and becomes a question when the source only
// says the person has an occupation.
func TestAgent_RegistryList(t *testing.T) {
	path := "personnes.pp[pp-1].activite.numeroIdentification.type"
	tc := agentPEPContext(t, map[string]any{"siren": "552100554", "occupation": "plombier"})
	siren := `{"path":"` + path + `","value":"SIREN","source":{"field":"siren","raw_value":"552100554"},"justification":"numéro SIREN","confidence":"medium"}`
	llm := newScriptedLLM(FinalStep(agFinal(siren)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, tc, agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "SIREN", out.Fills[0].Value)
	assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence)

	non := `{"path":"` + path + `","value":"Non","source":{"field":"occupation","raw_value":"plombier"},"justification":"exerce un métier","confidence":"medium"}`
	llm = newScriptedLLM(FinalStep(agFinal(non)))
	out, err = agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, tc, agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	assert.Empty(t, out.Fills)
	require.Len(t, out.Questions, 1)
}

// Sex list: SEX3 (Inconnu) is the unknown code, SEX1/SEX2 are strict. A gender
// column spelled with an alias proves the code; a first name does not.
func TestAgent_SexList(t *testing.T) {
	const sexePath = "personnes.pp[pp-1].etatCivil.sexe"
	fill := func(value, field, raw string) string {
		src := ""
		if field != "" {
			src = `,"source":{"field":"` + field + `","raw_value":"` + raw + `"}`
		}
		return `{"path":"` + sexePath + `","value":"` + value + `"` + src + `}`
	}
	run := func(f string, snap map[string]any) core.TaskOutput {
		llm := newScriptedLLM(FinalStep(agFinal(f)))
		out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, snap), agentSpec("fill", agentPersonPath))
		require.NoError(t, err)
		return out
	}
	out := run(fill("SEX1", "gender", "M"), map[string]any{"gender": "M"})
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "SEX1", out.Fills[0].Value)
	assert.Empty(t, out.Questions)

	out = run(fill("SEX2", "gender", "F"), map[string]any{"gender": "F"})
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "SEX2", out.Fills[0].Value)

	out = run(fill("SEX2", "gender", "féminin"), map[string]any{"gender": "féminin"})
	require.Len(t, out.Fills, 1, "alias, accent- and case-insensitive")

	out = run(fill("SEX1", "first_name", "Jean"), map[string]any{"first_name": "Jean"})
	assert.Empty(t, out.Fills, "a sex inferred from a first name is not a proof")
	require.Len(t, out.Questions, 1)
	var codes []string
	for _, o := range out.Questions[0].Options {
		codes = append(codes, o.Fills[0].Value)
	}
	assert.Equal(t, []string{"SEX1", "SEX2", "SEX3"}, codes)

	out = run(fill("SEX3", "", ""), nil)
	require.Len(t, out.Fills, 1)
	assert.Empty(t, out.Questions)
}

func TestFieldSpecAndDescribe_ShowUnknownCode(t *testing.T) {
	tc := agentPEPContext(t, nil)
	raw := toolsCall(t, CommonTools(tc), "field_spec", `{"path":"`+agPolitiqueePath+`"}`)
	m := toolsParseObj(t, raw)
	assert.Equal(t, "Je ne sais pas", m["unknown_code"])
	assert.Contains(t, m["unknown_code_rule"], "when the data say nothing")
	assert.Contains(t, m["unknown_code_rule"], `"Non" needs a source that establishes it`)

	d := DescribeSubtree(tc.Format.Spec(), tc.Doc, []core.Path{agentPersonPath})
	assert.Contains(t, d, `unknown code "Je ne sais pas": use it when the data say nothing`)
}

// ---------- B1: format instructions ----------

type instructedFormat struct {
	core.Format
	path string
}

func (f instructedFormat) InstructionsPromptPath() string { return f.path }

type mapPrompts struct{ texts map[string]string }

func (p mapPrompts) Exists(path string) bool { _, ok := p.texts[path]; return ok }
func (p mapPrompts) Read(path string) (string, error) {
	if t, ok := p.texts[path]; ok {
		return t, nil
	}
	return "", assert.AnError
}

func TestAgent_FormatInstructionsPrependedToStepPrompt(t *testing.T) {
	tc := agentToolContext(t)
	tc.Format = instructedFormat{Format: tc.Format, path: "fmt/format.md"}
	prompts := mapPrompts{texts: map[string]string{"prompts/fill.md": "STEP PROMPT", "fmt/format.md": "FORMAT RULES"}}
	llm := newScriptedLLM(FinalStep(agFinal()))
	out, err := agRun(t, llm, prompts, AgentLimits{}, tc, agentSpec("fill", agentPersonPath))
	require.NoError(t, err)

	prompt := llm.Requests[0].Prompt
	iConv := indexOf(prompt, "## Format conventions\nFORMAT RULES")
	iStep := indexOf(prompt, "STEP PROMPT")
	require.GreaterOrEqual(t, iConv, 0, prompt)
	require.Greater(t, iStep, iConv, "the conventions come before the step prompt")

	hdr := agTraceOf(t, out)[0]
	assert.Equal(t, "prompts/fill.md", hdr.PromptPath)
	assert.NotEmpty(t, hdr.PromptSHA256)
	assert.Equal(t, "fmt/format.md", hdr.FormatPromptPath)
	assert.NotEmpty(t, hdr.FormatPromptSHA256)
	assert.NotEqual(t, hdr.PromptSHA256, hdr.FormatPromptSHA256)
}

func TestAgent_FormatInstructionsMissingIsAnError(t *testing.T) {
	tc := agentToolContext(t)
	tc.Format = instructedFormat{Format: tc.Format, path: "fmt/format.md"}
	prompts := mapPrompts{texts: map[string]string{"prompts/fill.md": "STEP PROMPT"}}
	_, err := agRun(t, newScriptedLLM(FinalStep(agFinal())), prompts, AgentLimits{}, tc, agentSpec("fill", agentPersonPath))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fmt/format.md")
}

func TestAgent_FormatWithoutInstructionsIsUnchanged(t *testing.T) {
	tc := agentToolContext(t)
	tc.Format = instructedFormat{Format: tc.Format} // implements the interface, empty path
	llm := newScriptedLLM(FinalStep(agFinal()))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, tc, agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	assert.NotContains(t, llm.Requests[0].Prompt, "Format conventions")
	assert.Empty(t, agTraceOf(t, out)[0].FormatPromptPath)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// The orchestrator fails the job with « prompt manquant » when the format's
// instructions are missing, like a missing step prompt.
func TestRunJob_MissingFormatInstructionsFailsTheJob(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	f.Format.InstructionsPath = "fmt/format.md"
	f.Prompts.Missing["fmt/format.md"] = true
	f.Format.Waves = [][]core.TaskSpec{{Spec("a", tgtPP1)}}

	err := o.RunJob(ctx, f.JobId)
	require.Error(t, err)
	j := jobOf(t, f)
	assert.Equal(t, models.SarJobStatusFailed, j.Status)
	assert.Contains(t, j.Error, "prompt manquant : fmt/format.md")
}

// ---------- C: French reason for the analyst ----------

// After the last correction, a fill whose raw_value does not match the object
// becomes a question whose whole text is French (the correction sent to the
// model stays English).
func TestAgent_UnverifiableQuestionIsFrench(t *testing.T) {
	bad := `{"path":"` + agNomPath + `","value":"DUPONT","source":{"field":"last_name","raw_value":"Durand"}}`
	llm := newScriptedLLM(FinalStep(agFinal(bad)), FinalStep(agFinal(bad)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{MaxCorrections: 1}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)

	last := llm.Requests[1].History[len(llm.Requests[1].History)-1]
	assert.Contains(t, last.Text, "raw_value does not match", "the correction for the model stays English")

	require.Len(t, out.Questions, 1)
	assert.Equal(t, "valeur non vérifiable : "+agNomPath+" — la valeur brute citée ne correspond pas à celle de l'objet source", out.Questions[0].Text)
}

func TestFrenchReason_EveryCheckerErrorKindIsMapped(t *testing.T) {
	for en, fr := range map[string]string{
		"invalid path: x": "chemin invalide",
		"path outside the part assigned to this task; allowed paths: a":                                                "chemin hors de la partie confiée à cette tâche",
		"source without field (field)":                                                                                 "source sans champ",
		"table and object_id go together: give both or neither":                                                        "la table et l'identifiant de l'objet source vont ensemble",
		"the source object persons/p9 is not part of the selection":                                                    "l'objet source ne fait pas partie de la sélection",
		"the value does not match the cited source value (source value: \"x\")":                                        "la valeur ne correspond pas à celle de la source citée",
		"justify the correspondence between the source value and the chosen code (justification and confidence level)": "la correspondance avec la valeur de la source n'est pas justifiée",
		"justify the conversion from cents to euros (justification)":                                                   "la conversion des centimes en euros n'est pas justifiée",
		"une valeur recopiée doit citer au moins une source":                                                           "une valeur recopiée doit citer au moins une source",
		"valeur invalide (values that reference an element are applied after":                                          "valeur invalide",
	} {
		assert.Equal(t, fr, frenchReason(en), en)
	}
}

var _ = json.Marshal

// The analyst never reads the model's English wording or the internal name of a
// step: an unparsable fill error gets a generic French reason, and a limit on a
// task without target does not name the step kind.
func TestAgent_AnalystTextNeverShowsEnglishOrStepKind(t *testing.T) {
	tc := agentToolContext(t)
	spec := core.TaskSpec{Kind: "understanding", PromptPath: "prompts/understanding.md"}
	run := &agentRun{
		tc: tc, spec: spec, mode: modeFills, tr: newTrace("", ""),
		chk: newChecker(tc, spec, tc.TaskId, nil),
	}

	qs := run.unverifiableFromErrors([]string{"fills[3] without separator in English"})
	require.Len(t, qs, 1)
	assert.Equal(t, "valeur non vérifiable : la réponse du modèle n'a pas pu être exploitée", qs[0].Text)

	out := run.limitOutput("durée maximale atteinte")
	require.Len(t, out.Questions, 1)
	assert.NotContains(t, out.Questions[0].Text, "understanding")
	assert.Equal(t, "La génération automatique n'a pas abouti (durée maximale atteinte) : à vérifier et compléter manuellement.", out.Questions[0].Text)
}
