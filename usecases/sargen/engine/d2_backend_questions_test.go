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

const lpsPath = core.Path("context.information.activiteLPS")

func lpsDraft(doc *core.Document) []core.QuestionDraft {
	return []core.QuestionDraft{{
		Paths: []core.Path{lpsPath, "context.general.numeroProcedure"}, Text: "Rattacher ?", Blocking: true,
		Options: []core.QuestionOption{
			{Label: "Oui", Fills: []core.Fill{{Path: lpsPath, Value: "true"}}, Clear: []core.Path{"context.general.numeroProcedure"}},
			{Label: "Laisser"},
		},
	}}
}

func optionsOf(t *testing.T, q models.SarQuestion) []core.QuestionOption {
	t.Helper()
	var opts []core.QuestionOption
	if len(q.Options) > 0 {
		require.NoError(t, json.Unmarshal(q.Options, &opts))
	}
	return opts
}

// The backend questions of the format exist before the review task runs, are
// never written to the document, and the draft options are kept.
func TestBackendQuestions_ExistBeforeTheReviewAndWriteNothing(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, nil)
	f.Format.DocQuestionsFn = lpsDraft
	f.Format.Waves = [][]core.TaskSpec{{Spec(core.TaskKindReview, tgtPP1)}}
	var seenAtReview []models.SarQuestion
	f.Runner.On(core.TaskKindReview, tgtPP1, Step{Before: func() {
		seenAtReview = questionsOf(t, f, models.SarQuestionStatusOpen)
	}})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	require.Len(t, seenAtReview, 1, "asked before the review ran")
	q := seenAtReview[0]
	assert.Equal(t, models.SarQuestionKindQuestion, q.Kind)
	assert.Equal(t, models.SarQuestionSeverityBlocking, q.Severity)
	opts := optionsOf(t, q)
	require.Len(t, opts, 2)
	assert.Equal(t, []core.Path{"context.general.numeroProcedure"}, opts[0].Clear)
	_, has := valueOf(t, f, string(lpsPath))
	assert.False(t, has, "the question writes nothing by itself")
	assert.Len(t, questionsOf(t, f, ""), 1, "the job-end pass does not ask again")
}

// A group that already has a question (closed too) is not asked again.
func TestBackendQuestions_NotAskedTwice(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, nil)
	f.Format.DocQuestionsFn = lpsDraft
	require.NoError(t, f.Repo.CreateSarQuestions(ctx, nil, []models.SarQuestion{{
		DeclarationId: f.DeclId, Kind: models.SarQuestionKindQuestion, Status: models.SarQuestionStatusResolved,
		Paths: []string{"context.general.numeroProcedure", string(lpsPath)}, Text: "avant",
	}}))
	require.NoError(t, o.RunJob(ctx, f.JobId))
	assert.Len(t, questionsOf(t, f, ""), 1)
}

// An option that writes a setting is dropped; a draft whose writing options all
// fail is dropped as a whole.
func TestBackendQuestions_OptionsAreCheckedLikeAFill(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, nil)
	f.Format.DocQuestionsFn = func(*core.Document) []core.QuestionDraft {
		return []core.QuestionDraft{
			{
				Paths: []core.Path{"organisme.libelle"}, Text: "réglage ?",
				Options: []core.QuestionOption{{Label: "x", Fills: []core.Fill{{Path: "organisme.libelle", Value: "Pirate"}}}, {Label: "non"}},
			},
			{
				Paths: []core.Path{lpsPath}, Text: "hors périmètre ?",
				Options: []core.QuestionOption{
					{Label: "ok", Fills: []core.Fill{{Path: lpsPath, Value: "true"}}},
					{Label: "dehors", Fills: []core.Fill{{Path: "context.general.numeroProcedure", Value: "x"}}},
				},
			},
		}
	}
	require.NoError(t, o.RunJob(ctx, f.JobId))
	qs := questionsOf(t, f, "")
	require.Len(t, qs, 1, "the first draft is dropped")
	assert.Equal(t, "hors périmètre ?", qs[0].Text)
	require.Len(t, optionsOf(t, qs[0]), 1, "the option outside the paths is dropped")
}

// ---- D2.3 candidates in the data ----

type codeEnv struct {
	f    *fakes
	list string
}

func codeFixture(t *testing.T, semantic models.FieldSemanticType, snapshot map[string]any) (*Orchestrator, codeEnv) {
	t.Helper()
	o, f := newTestOrchestrator(t)
	node := f.Format.Spec().NodeAt("personnes.pm[].activiteCoordonnee.paysAutreTypeRegistre")
	require.NotNil(t, node)
	seedDocument(t, f, func(d *core.Document) {
		withForeignRegister("")(d)
		mustSetItemSources(d, "personnes.pm[pm-1]", []core.ObjectRef{{Table: "companies", ObjectID: "c1"}})
	})
	snapshot["object_id"] = "c1"
	f.Repo.selection = []models.SarSelectedObject{agentSelection("companies", "c1", snapshot)}
	fields := map[string]models.Field{"object_id": {Name: "object_id", DataType: models.String}}
	for k := range snapshot {
		if k != "object_id" {
			fields[k] = models.Field{Name: k, DataType: models.String, SemanticType: semantic}
		}
	}
	f.Repo.dataModel = models.DataModel{Tables: map[string]models.Table{"companies": {Name: "companies", Fields: fields}}}
	f.Format.CodeSemantics = map[string][]models.FieldSemanticType{node.CodeList: {models.FieldSemanticTypeCountry, models.FieldSemanticTypeIban}}
	f.Format.CodeFromValueFn = func(raw string) (string, bool) {
		switch raw {
		case "HK", "Hong Kong":
			return "HK", true
		case "FR7630006000011234567890189":
			return "FR", true
		}
		return "", false
	}
	return o, codeEnv{f: f, list: node.CodeList}
}

func TestMissingCode_CandidateInTheDataReplacesTheUnknownOption(t *testing.T) {
	for name, snap := range map[string]map[string]any{
		"country code":  {"country": "HK"},
		"country label": {"country": "Hong Kong"},
	} {
		t.Run(name, func(t *testing.T) {
			o, e := codeFixture(t, models.FieldSemanticTypeCountry, snap)
			require.NoError(t, o.RunJob(context.Background(), e.f.JobId))
			q, opts, ok := questionOn(t, e.f, pmRegistrePay)
			require.True(t, ok)
			require.Len(t, opts, 1, "one option for the candidate, no unknown code")
			assert.Equal(t, "HK", opts[0].Fills[0].Value)
			require.Len(t, opts[0].Fills[0].Sources, 1)
			assert.Equal(t, "companies", opts[0].Fills[0].Sources[0].Object.Table)
			assert.Equal(t, "country", opts[0].Fills[0].Sources[0].Field)
			assert.Contains(t, opts[0].Fills[0].Justification, "companies.country")
			assert.Contains(t, q.Text, "la sélection contient")
			assert.Contains(t, q.Text, "companies/c1, champ country")
			for _, op := range opts {
				assert.NotEqual(t, "Inconnu", op.Label)
			}
		})
	}
}

func TestMissingCode_ValidIbanGivesItsCountryPrefix(t *testing.T) {
	o, e := codeFixture(t, models.FieldSemanticTypeIban, map[string]any{"iban": "FR7630006000011234567890189"})
	require.NoError(t, o.RunJob(context.Background(), e.f.JobId))
	_, opts, ok := questionOn(t, e.f, pmRegistrePay)
	require.True(t, ok)
	require.Len(t, opts, 1)
	assert.Equal(t, "FR", opts[0].Fills[0].Value)
}

func TestMissingCode_UnmappableValueIsCitedWithoutOption(t *testing.T) {
	o, e := codeFixture(t, models.FieldSemanticTypeCountry, map[string]any{"country": "Atlantis"})
	require.NoError(t, o.RunJob(context.Background(), e.f.JobId))
	var q models.SarQuestion
	for _, c := range questionsOf(t, e.f, "") {
		if len(c.Paths) == 1 && c.Paths[0] == pmRegistrePay {
			q = c
		}
	}
	require.NotEmpty(t, q.Text)
	assert.Empty(t, optionsOf(t, q), "no option, no unknown code")
	assert.Contains(t, q.Text, "Atlantis")
}

func TestMissingCode_NoSemanticTypeKeepsTheUnknownOption(t *testing.T) {
	o, e := codeFixture(t, models.FieldSemanticTypeUnset, map[string]any{"country": "HK"})
	require.NoError(t, o.RunJob(context.Background(), e.f.JobId))
	_, opts, ok := questionOn(t, e.f, pmRegistrePay)
	require.True(t, ok)
	require.Len(t, opts, 1)
	assert.Equal(t, "Inconnu", opts[0].Label)
}
