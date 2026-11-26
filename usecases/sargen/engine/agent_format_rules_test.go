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

// « Non » from a KYC risk level « low » is a weak relation: without
// justification it is sent back, with one it is accepted, confidence forced to
// low, and no question is created.
func TestAgent_UnknownCodeList_WeakRelationNeedsJustification(t *testing.T) {
	bare := pepFill("Non", "kyc_risk", "low", "")
	justified := pepFill("Non", "kyc_risk", "low", `,"justification":"risque faible","confidence":"medium"`)
	llm := newScriptedLLM(FinalStep(agFinal(bare)), FinalStep(agFinal(justified)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, nil), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	require.Equal(t, 2, llm.Calls(), "the unjustified answer is sent back once")
	agLastIsCorrection(t, llm.Requests[1])
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "Non", out.Fills[0].Value)
	assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence)
	assert.Empty(t, out.Questions)
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

// A boolean data-model field proves a yes/no code with no judgment; so do the
// code itself or its label.
func TestAgent_UnknownCodeList_BooleanSourceAccepted(t *testing.T) {
	t.Run("boolean source", func(t *testing.T) {
		llm := newScriptedLLM(FinalStep(agFinal(pepFill("Oui", agPEPContextField, "true", ""))))
		out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, map[string]any{agPEPContextField: true}), agentSpec("fill", agentPersonPath))
		require.NoError(t, err)
		assert.Equal(t, 1, llm.Calls(), "no correction round")
		require.Len(t, out.Fills, 1)
		assert.Equal(t, "Oui", out.Fills[0].Value)
		assert.Equal(t, core.ConfidenceNone, out.Fills[0].Confidence, "confidence is not forced to low")
		assert.Empty(t, out.Questions)
	})
	t.Run("boolean source disagreeing", func(t *testing.T) {
		wrong := pepFill("Non", agPEPContextField, "true", "")
		right := pepFill("Oui", agPEPContextField, "true", "")
		llm := newScriptedLLM(FinalStep(agFinal(wrong)), FinalStep(agFinal(right)))
		out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, map[string]any{agPEPContextField: true}), agentSpec("fill", agentPersonPath))
		require.NoError(t, err)
		require.Equal(t, 2, llm.Calls(), "the contradiction is sent back")
		agLastIsCorrection(t, llm.Requests[1])
		require.Len(t, out.Fills, 1)
		assert.Equal(t, "Oui", out.Fills[0].Value)
	})
	t.Run("raw value is the code", func(t *testing.T) {
		llm := newScriptedLLM(FinalStep(agFinal(pepFill("Non", "pep_status", "Non", ""))))
		out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, map[string]any{"pep_status": "Non"}), agentSpec("fill", agentPersonPath))
		require.NoError(t, err)
		assert.Equal(t, 1, llm.Calls())
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
// in the data model: a count of 1 or a score of 0 prove nothing and need a
// justification.
func TestAgent_BooleanLiteralNeedsBooleanField(t *testing.T) {
	llm := newScriptedLLM(FinalStep(agFinal(pepFill("Oui", "nb_flags", "1", `,"justification":"un signalement","confidence":"medium"`))))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, map[string]any{"nb_flags": 1}), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	require.Len(t, out.Fills, 1, "a weak relation with a justification is accepted")
	assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence)

	bare := pepFill("Non", "nb_flags", "0", "")
	justified := pepFill("Non", "nb_flags", "0", `,"justification":"aucun signalement","confidence":"medium"`)
	llm = newScriptedLLM(FinalStep(agFinal(bare)), FinalStep(agFinal(justified)))
	out, err = agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, map[string]any{"nb_flags": 0}), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	require.Equal(t, 2, llm.Calls(), "Non from a numeric 0 needs a justification")
	agLastIsCorrection(t, llm.Requests[1])
	require.Len(t, out.Fills, 1)
	assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence)
	assert.Empty(t, out.Questions)
}

// A weak source with a justification is accepted, low.
func TestAgent_CodeWeakSourceAccepted(t *testing.T) {
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
// is accepted (low); so is Non from an occupation, with a justification, and no
// question is created.
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
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "Non", out.Fills[0].Value)
	assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence)
	assert.Empty(t, out.Questions)
}

// Sex list: SEX3 (Inconnu) is the unknown code, SEX1 is labelled "H" and SEX2
// "F". A raw value equal to the label needs no judgment; anything else
// (another letter, another language, a first name) needs a justification.
func TestAgent_SexList(t *testing.T) {
	const sexePath = "personnes.pp[pp-1].etatCivil.sexe"
	fill := func(value, field, raw, extra string) string {
		src := ""
		if field != "" {
			src = `,"source":{"field":"` + field + `","raw_value":"` + raw + `"}`
		}
		return `{"path":"` + sexePath + `","value":"` + value + `"` + src + extra + `}`
	}
	const just = `,"justification":"déduit de la valeur","confidence":"medium"`
	run := func(snap map[string]any, steps ...string) (core.TaskOutput, int) {
		var ss []ScriptedStep
		for _, f := range steps {
			ss = append(ss, FinalStep(agFinal(f)))
		}
		llm := newScriptedLLM(ss...)
		out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, snap), agentSpec("fill", agentPersonPath))
		require.NoError(t, err)
		return out, llm.Calls()
	}

	out, calls := run(map[string]any{"gender": "F"}, fill("SEX2", "gender", "F", ""))
	assert.Equal(t, 1, calls, "the label is a proof: no judgment")
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "SEX2", out.Fills[0].Value)
	assert.Equal(t, core.ConfidenceNone, out.Fills[0].Confidence)

	for _, c := range []struct{ value, field, raw string }{
		{"SEX1", "gender", "M"},
		{"SEX2", "gender", "féminin"},
		{"SEX1", "first_name", "Jean"},
	} {
		snap := map[string]any{c.field: c.raw}
		out, calls = run(snap, fill(c.value, c.field, c.raw, ""), fill(c.value, c.field, c.raw, just))
		assert.Equal(t, 2, calls, c.raw+": a justification is required")
		require.Len(t, out.Fills, 1, c.raw)
		assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence, c.raw)
		assert.Empty(t, out.Questions, c.raw)
	}

	out, calls = run(nil, fill("SEX3", "", "", ""))
	assert.Equal(t, 1, calls)
	require.Len(t, out.Fills, 1)
	assert.Empty(t, out.Questions)
}

// A raw value in another language is accepted only with a justification, and the
// engine then forces the confidence to low and creates no question.
func TestAgent_SexList_ForeignWordNeedsJustification(t *testing.T) {
	const sexePath = "personnes.pp[pp-1].etatCivil.sexe"
	src := `"source":{"field":"gender","raw_value":"masculino"}`
	bare := `{"path":"` + sexePath + `","value":"SEX1",` + src + `}`
	justified := `{"path":"` + sexePath + `","value":"SEX1",` + src + `,"justification":"masculino signifie masculin","confidence":"medium"}`
	llm := newScriptedLLM(FinalStep(agFinal(bare)), FinalStep(agFinal(justified)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, map[string]any{"gender": "masculino"}), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	require.Equal(t, 2, llm.Calls(), "without a justification the engine asks for one")
	agLastIsCorrection(t, llm.Requests[1])
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "SEX1", out.Fills[0].Value)
	assert.Equal(t, core.ConfidenceLow, out.Fills[0].Confidence, "medium is forced to low")
	assert.Empty(t, out.Questions)
}

func TestFieldSpecAndDescribe_ShowUnknownCode(t *testing.T) {
	tc := agentPEPContext(t, nil)
	raw := toolsCall(t, CommonTools(tc), "field_spec", `{"path":"`+agPolitiqueePath+`"}`)
	m := toolsParseObj(t, raw)
	assert.Equal(t, "Je ne sais pas", m["unknown_code"])
	assert.Contains(t, m["unknown_code_rule"], "when the data say nothing")
	assert.NotContains(t, m["unknown_code_rule"], "strict")
	assert.NotContains(t, m["unknown_code_rule"], "needs a source that establishes it")

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
		"a copied value must cite at least one source":                                                                 "une valeur recopiée doit citer au moins une source",
		"invalid date, expected format YYYY-MM-DD (values that reference an element are applied after":                 "date invalide (AAAA-MM-JJ attendu)",
		"at most 5 items (6 present)":                                                                                  "valeur trop longue ou nombre maximal atteint",
		"The estimated birth date is filled only when the precise one is absent":                                       "valeur refusée par le format",
		"write the code FR, not its label or another spelling of it":                                                   "le code de la liste doit être écrit tel quel, et non son libellé",
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

// A copied code other than the unknown code still needs a source: "Non" with
// none is sent back to the model, never applied as is.
func TestAgent_UnknownCodeList_CopiedCodeWithoutSourceSentBack(t *testing.T) {
	bare := `{"path":"` + agPolitiqueePath + `","value":"Non"}`
	unknown := `{"path":"` + agPolitiqueePath + `","value":"Je ne sais pas"}`
	llm := newScriptedLLM(FinalStep(agFinal(bare)), FinalStep(agFinal(unknown)))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentPEPContext(t, nil), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)
	require.Equal(t, 2, llm.Calls(), "the answer without source is sent back once")
	agLastIsCorrection(t, llm.Requests[1])
	require.Len(t, out.Fills, 1)
	assert.Equal(t, "Je ne sais pas", out.Fills[0].Value)
	assert.Empty(t, out.Questions)
}
