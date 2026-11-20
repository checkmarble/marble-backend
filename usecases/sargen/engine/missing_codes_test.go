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

const (
	pmRegistrePay = "personnes.pm[pm-1].activiteCoordonnee.paysAutreTypeRegistre"
)

// withForeignRegister seeds a legal entity whose register is foreign (RN6): the
// format then requires the country of the register (paysAutreTypeRegistre, a
// country code, list with an unknown code).
func withForeignRegister(country string) func(d *core.Document) {
	return func(d *core.Document) {
		pm := d.AddItem("personnes.pm", "pm")
		d.Values[pm.Child("activiteCoordonnee").Child("numeroIdentification").Child("type")] = core.Value{Value: "RN6", Origin: core.OriginLLM}
		if country != "" {
			d.Values[core.Path(pmRegistrePay)] = core.Value{Value: country, Origin: core.OriginLLM}
		}
	}
}

func finishJob(t *testing.T, seed func(d *core.Document)) *fakes {
	t.Helper()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, seed)
	require.NoError(t, o.RunJob(context.Background(), f.JobId))
	require.Equal(t, models.SarJobStatusDone, jobOf(t, f).Status)
	return f
}

func questionOn(t *testing.T, f *fakes, path string) (models.SarQuestion, []core.QuestionOption, bool) {
	t.Helper()
	for _, q := range questionsOf(t, f, "") {
		for _, p := range q.Paths {
			if p == path {
				var opts []core.QuestionOption
				require.NoError(t, json.Unmarshal(q.Options, &opts))
				return q, opts, true
			}
		}
	}
	return models.SarQuestion{}, nil, false
}

// An empty required country of a list with an unknown code gets a question whose
// option fills the unknown code (here the country of a foreign register, required
// by a conditional rule).
func TestFinish_EmptyRequiredCountryOffersTheUnknownCode(t *testing.T) {
	f := finishJob(t, withForeignRegister(""))

	q, opts, ok := questionOn(t, f, pmRegistrePay)
	require.True(t, ok, "a question on the empty country")
	assert.Equal(t, models.SarQuestionKindQuestion, q.Kind)
	require.Len(t, opts, 1)
	assert.Equal(t, "Inconnu", opts[0].Label)
	require.Len(t, opts[0].Fills, 1)
	assert.Equal(t, core.Path(pmRegistrePay), opts[0].Fills[0].Path)
	assert.Equal(t, "XX", opts[0].Fills[0].Value)
	assert.Contains(t, q.Text, "valeur manquante")
}

func TestFinish_FilledCountryGetsNoQuestion(t *testing.T) {
	f := finishJob(t, withForeignRegister("FR"))
	_, _, ok := questionOn(t, f, pmRegistrePay)
	assert.False(t, ok)
}

// A field the analyst cleared by hand is left alone, and a field that already has
// an open question is not asked twice.
func TestFinish_ClearedOrAlreadyAskedFieldGetsNoQuestion(t *testing.T) {
	f := finishJob(t, func(d *core.Document) {
		withForeignRegister("")(d)
		d.Values[pmRegistrePay] = core.Value{Value: "", Origin: core.OriginManual}
	})
	_, _, ok := questionOn(t, f, pmRegistrePay)
	assert.False(t, ok, "cleared by hand")

	o, f2 := newTestOrchestrator(t)
	seedDocument(t, f2, withForeignRegister(""))
	require.NoError(t, f2.Repo.CreateSarQuestions(context.Background(), nil, []models.SarQuestion{{
		DeclarationId: f2.DeclId, Kind: models.SarQuestionKindQuestion, Paths: []string{pmRegistrePay},
		Text: "pays ?", Status: models.SarQuestionStatusOpen,
	}}))
	require.NoError(t, o.RunJob(context.Background(), f2.JobId))
	assert.Len(t, questionsOf(t, f2, ""), 1, "no second question on the same path")
}

// A coded field whose list has no unknown code is never asked about.
func TestFinish_ListWithoutUnknownCodeGetsNoQuestion(t *testing.T) {
	f := finishJob(t, func(d *core.Document) {
		tx := d.AddItem("work.transactions", "transactions")
		d.Values[tx.Child("montant")] = core.Value{Value: "10", Origin: core.OriginLLM}
		// devise (required, a list without unknown code) is empty.
	})
	for _, q := range questionsOf(t, f, "") {
		assert.NotContains(t, q.Paths, "work.transactions[transactions-1].devise")
	}
}

// A question that removes an item: the item must be one of the question's paths
// and exist; the keep option (no fill) survives; blocking is stored.
func TestRunJob_RemoveOptionKeptWithItsKeepSiblingAndBlockingSeverity(t *testing.T) {
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	f.Format.Waves = [][]core.TaskSpec{{Spec("a", tgtPP1)}}
	f.Runner.On("a", tgtPP1, Step{Out: core.TaskOutput{Questions: []core.QuestionDraft{{
		Paths: []core.Path{tgtPP1}, Text: "Déclarer ?", Blocking: true,
		Options: []core.QuestionOption{{Label: "Déclarer"}, {Label: "Ne pas déclarer", RemoveItem: tgtPP1}},
	}}}})
	require.NoError(t, o.RunJob(context.Background(), f.JobId))

	q, opts, ok := questionOn(t, f, tgtPP1)
	require.True(t, ok)
	assert.Equal(t, models.SarQuestionSeverityBlocking, q.Severity)
	require.Len(t, opts, 2)
	assert.Empty(t, opts[0].RemoveItem)
	assert.Equal(t, core.Path(tgtPP1), opts[1].RemoveItem)
}

// A question whose removing option is refused is dropped as a whole: no blocking
// question is ever left without its options.
func TestRunJob_QuestionWithARefusedRemovalOptionIsDropped(t *testing.T) {
	for name, remove := range map[string]core.Path{"not a path of the question": tgtPP2, "missing item": "personnes.pp[pp-9]"} {
		t.Run(name, func(t *testing.T) {
			o, f := newTestOrchestrator(t)
			seedDocument(t, f, withTwoPersons)
			f.Format.Waves = [][]core.TaskSpec{{Spec("a", tgtPP1)}}
			f.Runner.On("a", tgtPP1, Step{Out: core.TaskOutput{Questions: []core.QuestionDraft{{
				Paths: []core.Path{tgtPP1}, Text: "Déclarer ?", Blocking: true,
				Options: []core.QuestionOption{{Label: "Déclarer"}, {Label: "Ne pas déclarer", RemoveItem: remove}},
			}}}})
			require.NoError(t, o.RunJob(context.Background(), f.JobId))

			assert.Empty(t, questionsOf(t, f, ""), "no question at all")
		})
	}
}
