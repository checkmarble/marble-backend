package sargen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Task 11 bis: questions (list, choose an option, dismiss) and the todo entries
// they produce.

// genQuestionById returns the question with the given id among qs.
func genQuestionById(t *testing.T, qs []Question, id uuid.UUID) Question {
	t.Helper()
	for _, q := range qs {
		if q.Id == id {
			return q
		}
	}
	require.Failf(t, "question not listed", "question %s", id)
	return Question{}
}

// findTodo returns the first todo item matching pred.
func findTodo(todo []TodoItem, pred func(TodoItem) bool) (TodoItem, bool) {
	for _, item := range todo {
		if pred(item) {
			return item, true
		}
	}
	return TodoItem{}, false
}

func todoRefIs(kind TodoKind, id uuid.UUID) func(TodoItem) bool {
	return func(item TodoItem) bool {
		return item.Kind == kind && item.RefId != nil && *item.RefId == id
	}
}

// ---- Manual edit resolves a question ----

func TestManualEditResolvesQuestion(t *testing.T) {
	t.Run("filling the only path resolves the question", func(t *testing.T) {
		uc, f, declId := genReady(t, false)
		q := f.SeedQuestion(t, declId, []core.Path{"organisme.libelle"}, "Quel est le libellé ?", nil)

		_, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
			[]core.Fill{{Path: "organisme.libelle", Value: "Banque Test"}})
		require.NoError(t, err)

		qs, err := uc.ListQuestions(f.Ctx, declId)
		require.NoError(t, err)
		assert.Equal(t, models.SarQuestionStatusResolved, genQuestionById(t, qs, q.Id).Status)
	})

	t.Run("the question stays open until every path is filled", func(t *testing.T) {
		uc, f, declId := genReady(t, false)
		q := f.SeedQuestion(t, declId, []core.Path{"organisme.libelle", "organisme.numIdPro"}, "Identité de l'organisme ?", nil)

		_, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
			[]core.Fill{{Path: "organisme.libelle", Value: "Banque Test"}})
		require.NoError(t, err)
		qs, err := uc.ListQuestions(f.Ctx, declId)
		require.NoError(t, err)
		assert.Equal(t, models.SarQuestionStatusOpen, genQuestionById(t, qs, q.Id).Status)

		_, err = uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
			[]core.Fill{{Path: "organisme.numIdPro", Value: "123456789"}})
		require.NoError(t, err)
		qs, err = uc.ListQuestions(f.Ctx, declId)
		require.NoError(t, err)
		assert.Equal(t, models.SarQuestionStatusResolved, genQuestionById(t, qs, q.Id).Status)
	})
}

// ---- Choose an option ----

// genOptionWithFill is an option applying one data-model value with its source.
func genOptionWithFill() []core.QuestionOption {
	return []core.QuestionOption{{
		Label: "Oui",
		Fills: []core.Fill{{
			Path:       "context.general.estRequisitionJudiciaire",
			Value:      "true",
			Origin:     core.OriginDataModel,
			Sources:    []core.SourceRef{{Object: core.ObjectRef{Table: "transactions", ObjectID: "t1"}, Field: "requisition", RawValue: "true"}},
			Confidence: core.ConfidenceHigh,
		}},
	}, {
		Label: "Non",
	}}
}

func TestChooseQuestionOption_AppliesFillsWithoutJob(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{"context.general.estRequisitionJudiciaire"},
		"S'agit-il d'une réquisition judiciaire ?", genOptionWithFill())
	version := f.Repo.Declarations[declId].Version

	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, version, 0)
	require.NoError(t, err)

	v, ok := st.Doc.Values["context.general.estRequisitionJudiciaire"]
	require.True(t, ok, "the option value must be applied")
	assert.Equal(t, "true", v.Value)
	assert.Equal(t, core.OriginDataModel, v.Origin, "the origin of the option is kept")
	assert.True(t, v.Reviewed, "the applied value is marked reviewed")
	require.Len(t, v.Sources, 1, "the sources of the option are kept")
	assert.Equal(t, "t1", v.Sources[0].Object.ObjectID)

	qs, err := uc.ListQuestions(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusResolved, genQuestionById(t, qs, q.Id).Status)

	assert.Empty(t, f.JobsOf(declId), "no job must be created")
	assert.Empty(t, f.Enqueuer.Calls, "nothing must be enqueued")
}

func TestChooseQuestionOption_RefusedWithoutFills(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{"organisme.libelle"}, "Quel est le libellé ?",
		[]core.QuestionOption{{Label: "Je ne sais pas"}})
	version := f.Repo.Declarations[declId].Version

	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, version, 0)
	require.ErrorIs(t, err, models.BadParameterError, "an option without fills cannot be chosen")

	_, err = uc.ChooseQuestionOption(f.Ctx, q.Id, version, 7)
	require.ErrorIs(t, err, models.BadParameterError, "an out of range option is refused")

	qs, err := uc.ListQuestions(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusOpen, genQuestionById(t, qs, q.Id).Status)
}

func TestChooseQuestionOption_OtherOrgForbidden(t *testing.T) {
	uc, f := newTestUsecase(t)
	foreign := models.SarDeclaration{
		Id:             uuid.New(),
		OrganizationId: uuid.New(), // another organization
		CaseId:         f.CaseId,
		FormatKey:      tracfinFormatKey,
		ReportType:     tracfinReportType,
		Version:        1,
		Document:       []byte(`{"values":{},"items":{},"item_sources":{},"next_key":{}}`),
	}
	require.NoError(t, f.Repo.CreateSarDeclaration(f.Ctx, nil, foreign))
	docBefore := append([]byte(nil), foreign.Document...)
	q := f.SeedQuestion(t, foreign.Id, []core.Path{"context.general.estRequisitionJudiciaire"},
		"Réquisition ?", genOptionWithFill())

	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, 1, 0)
	require.ErrorIs(t, err, models.ForbiddenError)
	assert.True(t, bytes.Equal(docBefore, f.Repo.Declarations[foreign.Id].Document), "the foreign document must not change")
}

// ---- Dismiss ----

func TestDismissQuestion_ClosesFinding(t *testing.T) {
	uc, f, declId := genReady(t, false)
	finding := f.SeedReviewFinding(t, declId, []core.Path{"organisme.libelle"}, "important", "Libellé à vérifier")

	qs, err := uc.ListQuestions(f.Ctx, declId)
	require.NoError(t, err)
	listed := genQuestionById(t, qs, finding.Id)
	assert.Equal(t, models.SarQuestionKindReviewFinding, listed.Kind)
	assert.Equal(t, "important", listed.Severity)
	assert.Equal(t, models.SarQuestionStatusOpen, listed.Status)

	require.NoError(t, uc.DismissQuestion(f.Ctx, finding.Id))

	qs, err = uc.ListQuestions(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusDismissed, genQuestionById(t, qs, finding.Id).Status)

	err = uc.DismissQuestion(f.Ctx, finding.Id)
	require.ErrorIs(t, err, models.ConflictError, "a closed finding cannot be dismissed again")
}

// ---- Todo ----

// TestBuildTodo_QuestionsFindingsAndComputedToConfirm checks the todo entries
// through the declaration state (GetDeclaration).
func TestBuildTodo_QuestionsFindingsAndComputedToConfirm(t *testing.T) {
	uc, f, declId := genReady(t, true)
	// operations.montantTotal is a computed value that the analyst has not confirmed.
	genEditDoc(t, f, declId, func(doc *core.Document) {
		doc.Values["operations.montantTotal"] = core.Value{Value: "12500.00", Origin: core.OriginComputed}
	})
	blockingQ := f.SeedQuestion(t, declId, []core.Path{"organisme.libelle"}, "Quel est le libellé ?", nil)
	warningQ := f.SeedQuestion(t, declId, []core.Path{"organisme.adresse.numero"}, "Numéro de voie ?", nil)
	finding := f.SeedReviewFinding(t, declId, []core.Path{"operations.montantTotal"}, "important", "Montant à vérifier")

	st, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)

	item, ok := findTodo(st.Todo, todoRefIs(TodoQuestion, blockingQ.Id))
	require.True(t, ok, "an open question must appear in the todo")
	assert.Equal(t, TodoBlocking, item.Severity, "a question on a required path blocks the export")

	item, ok = findTodo(st.Todo, todoRefIs(TodoQuestion, warningQ.Id))
	require.True(t, ok)
	assert.Equal(t, TodoWarning, item.Severity, "a question on an optional path is a warning")

	item, ok = findTodo(st.Todo, todoRefIs(TodoReviewFinding, finding.Id))
	require.True(t, ok, "an open review finding must appear in the todo")
	assert.Equal(t, TodoReview, item.Severity)
	assert.NotEqual(t, TodoBlocking, item.Severity, "a finding never blocks")

	computed := func(item TodoItem) bool {
		return item.Severity == TodoReview &&
			strings.Contains(item.Message, "valeur calculée à confirmer") &&
			containsPath(item.Paths, "operations.montantTotal")
	}
	_, ok = findTodo(st.Todo, computed)
	assert.True(t, ok, "an unconfirmed computed value must be a review entry")

	genEditDoc(t, f, declId, func(doc *core.Document) {
		v := doc.Values["operations.montantTotal"]
		v.Reviewed = true
		doc.Values["operations.montantTotal"] = v
	})
	st, err = uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	_, ok = findTodo(st.Todo, computed)
	assert.False(t, ok, "a confirmed computed value leaves the todo")
}

func containsPath(paths []core.Path, want core.Path) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}
