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
		q := f.SeedQuestion(t, declId, []core.Path{"organisme.libelle", "organisme.adresse.numero"}, "Identité de l'organisme ?", nil)

		_, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
			[]core.Fill{{Path: "organisme.libelle", Value: "Banque Test"}})
		require.NoError(t, err)
		qs, err := uc.ListQuestions(f.Ctx, declId)
		require.NoError(t, err)
		assert.Equal(t, models.SarQuestionStatusOpen, genQuestionById(t, qs, q.Id).Status)

		_, err = uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
			[]core.Fill{{Path: "organisme.adresse.numero", Value: "12"}})
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
			Path:       "context.information.activiteLPS",
			Value:      "true",
			Sources:    []core.SourceRef{{Object: core.ObjectRef{Table: "transactions", ObjectID: "t1"}, Field: "requisition", RawValue: "true"}},
			Confidence: core.ConfidenceHigh, Justification: "indiqué dans la transaction",
		}},
	}, {
		Label: "Non",
	}}
}

func TestChooseQuestionOption_AppliesFillsWithoutJob(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{"context.information.activiteLPS"},
		"L'organisme exerce-t-il en LPS ?", genOptionWithFill())
	version := f.Repo.Declarations[declId].Version

	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, version, 0)
	require.NoError(t, err)

	v, ok := st.Doc.Values["context.information.activiteLPS"]
	require.True(t, ok, "the option value must be applied")
	assert.Equal(t, "true", v.Value)
	assert.Equal(t, core.OriginAnalyst, v.Origin, "an option is the analyst's choice")
	assert.Nil(t, v.TaskID)
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
	q := f.SeedQuestion(t, foreign.Id, []core.Path{"context.information.activiteLPS"},
		"LPS ?", genOptionWithFill())

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
	item, ok = findTodo(st.Todo, computed)
	assert.True(t, ok, "an unconfirmed computed value must be a review entry")
	assert.Equal(t, TodoCodeComputedToConfirm, item.Code, "stable code for clients")

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

// ---- Option fills are confined to the question's paths and to LLM rules ----

func genChoose(t *testing.T, uc *Usecase, f *fakes, declId uuid.UUID, paths []core.Path, opts []core.QuestionOption) (models.SarQuestion, error) {
	t.Helper()
	q := f.SeedQuestion(t, declId, paths, "Question ?", opts)
	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, 0)
	return q, err
}

func TestChooseQuestionOption_ForgedOriginAndForeignPathsRefused(t *testing.T) {
	cases := []struct {
		name  string
		paths []core.Path
		fill  core.Fill
	}{
		{"forged origin on a setting", []core.Path{"organisme.libelle"},
			core.Fill{Path: "organisme.libelle", Value: "Pirate", Origin: core.OriginOrgSetting}},
		{"forged origin on a computed path", []core.Path{"operations.montantTotal"},
			core.Fill{Path: "operations.montantTotal", Value: "1.00", Origin: core.OriginComputed}},
		{"path outside the question", []core.Path{"context.information.activiteLPS"},
			core.Fill{Path: "context.information.lienTransportDeFond", Value: "true"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uc, f, declId := genReady(t, false)
			docBefore := append([]byte(nil), f.Repo.Declarations[declId].Document...)
			q, err := genChoose(t, uc, f, declId, c.paths, []core.QuestionOption{{Label: "x", Fills: []core.Fill{c.fill}}})
			require.ErrorIs(t, err, models.BadParameterError)
			assert.True(t, bytes.Equal(docBefore, f.Repo.Declarations[declId].Document), "nothing applied")
			assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
		})
	}
}

func TestChooseQuestionOption_EmptyValueClearsWithoutError(t *testing.T) {
	uc, f, declId := genReady(t, false)
	_, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
		[]core.Fill{{Path: "context.general.numeroProcedure", Value: "P-1"}})
	require.NoError(t, err)
	q, err := genChoose(t, uc, f, declId, []core.Path{"context.general.numeroProcedure"},
		[]core.QuestionOption{{Label: "Aucun", Fills: []core.Fill{{Path: "context.general.numeroProcedure", Value: "",
			Sources: []core.SourceRef{{Object: core.ObjectRef{Table: "transactions", ObjectID: "t1"}, Field: "requisition"}}}}}})
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
	doc := decodeDoc(t, f.Repo.Declarations[declId].Document)
	v, ok := doc.Values["context.general.numeroProcedure"]
	assert.True(t, !ok || v.Value == "", "the value must be cleared")
}

func TestChooseQuestionOption_PathlessQuestionCannotWrite(t *testing.T) {
	uc, f, declId := genReady(t, false)
	docBefore := append([]byte(nil), f.Repo.Declarations[declId].Document...)
	q, err := genChoose(t, uc, f, declId, nil, []core.QuestionOption{{Label: "x",
		Fills: []core.Fill{{Path: "organisme.libelle", Value: "Pirate"}}}})
	require.ErrorIs(t, err, models.BadParameterError)
	assert.True(t, bytes.Equal(docBefore, f.Repo.Declarations[declId].Document))
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
}

func TestChooseQuestionOption_RefusedDuringJob(t *testing.T) {
	uc, f, declId := genReady(t, false)
	f.SeedJob(t, declId, models.SarJobKindReview, models.SarJobStatusRunning)
	q := f.SeedQuestion(t, declId, []core.Path{"context.information.activiteLPS"}, "LPS ?", genOptionWithFill())
	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, 0)
	require.ErrorIs(t, err, ErrGenerationInProgress)
}

// ---- Other organization ----

func TestQuestions_OtherOrgForbidden(t *testing.T) {
	uc, f := newTestUsecase(t)
	foreign := models.SarDeclaration{
		Id: uuid.New(), OrganizationId: uuid.New(), CaseId: f.CaseId,
		FormatKey: tracfinFormatKey, ReportType: tracfinReportType, Version: 1,
		Document: []byte(`{"values":{},"items":{},"item_sources":{},"next_key":{}}`),
	}
	require.NoError(t, f.Repo.CreateSarDeclaration(f.Ctx, nil, foreign))
	q := f.SeedReviewFinding(t, foreign.Id, nil, "mineur", "x")
	failedJob := f.SeedJob(t, foreign.Id, models.SarJobKindGenerate, models.SarJobStatusFailed)

	_, err := uc.ListQuestions(f.Ctx, foreign.Id)
	require.ErrorIs(t, err, models.ForbiddenError)
	require.ErrorIs(t, uc.DismissQuestion(f.Ctx, q.Id), models.ForbiddenError)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
	_, err = uc.StartGeneration(f.Ctx, foreign.Id)
	require.ErrorIs(t, err, models.ForbiddenError)
	_, err = uc.StartReview(f.Ctx, foreign.Id)
	require.ErrorIs(t, err, models.ForbiddenError)
	_, err = uc.ResumeJob(f.Ctx, failedJob.Id)
	require.ErrorIs(t, err, models.ForbiddenError)
	assert.Empty(t, f.Enqueuer.Calls)
	assert.Equal(t, models.SarJobStatusFailed, f.Gen.Jobs[failedJob.Id].Status)
}

func TestChooseQuestionOption_WithoutSourceAppliesAsAnalyst(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{"context.information.activiteLPS"}, "LPS ?",
		[]core.QuestionOption{{Label: "Oui", Fills: []core.Fill{{Path: "context.information.activiteLPS", Value: "true"}}}})
	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, 0)
	require.NoError(t, err)
	v := st.Doc.Values["context.information.activiteLPS"]
	assert.Equal(t, "true", v.Value)
	assert.Equal(t, core.OriginAnalyst, v.Origin)
	assert.True(t, v.Reviewed)
	assert.Empty(t, v.Sources)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
}

// ---- Options that add a list item ----

const genPenalPath = core.Path("context.natureSoupcon.natureSoupconPenale[+a]")

// genAddOptions is the « add / do not add » pair of a question on a new list item.
func genAddOptions(path core.Path, code string) []core.QuestionOption {
	return []core.QuestionOption{
		{Label: "Oui, déclarer", Fills: []core.Fill{{Path: path, Value: code, Confidence: core.ConfidenceHigh}}},
		{Label: "Non, ne pas déclarer"},
	}
}

func TestChooseQuestionOption_AddOptionCreatesItem(t *testing.T) {
	uc, f, declId := genReady(t, false)
	// The penal nature is required by the legal framework CL02.
	_, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
		[]core.Fill{{Path: "context.natureSoupcon.cadreLegal[+c]", Value: "CL02"}})
	require.NoError(t, err)
	q := f.SeedQuestion(t, declId, []core.Path{genPenalPath}, "Faut-il déclarer l'escroquerie ?",
		genAddOptions(genPenalPath, "NSP07"))
	version := f.Repo.Declarations[declId].Version

	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, version, 0)
	require.NoError(t, err)

	keys := st.Doc.Items["context.natureSoupcon.natureSoupconPenale"]
	require.Len(t, keys, 1, "one item must be created")
	p := core.Path("context.natureSoupcon.natureSoupconPenale[" + keys[0] + "]")
	v := st.Doc.Values[p]
	assert.Equal(t, "NSP07", v.Value)
	assert.Equal(t, core.OriginAnalyst, v.Origin)
	assert.True(t, v.Reviewed)
	assert.Nil(t, v.TaskID)
	assert.Equal(t, version+1, st.Declaration.Version)
	for _, is := range st.Issues {
		assert.NotContains(t, string(is.Path), "natureSoupconPenale", "the created item must be valid")
	}
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)

	_, err = uc.ChooseQuestionOption(f.Ctx, q.Id, st.Declaration.Version, 0)
	require.ErrorIs(t, err, models.ConflictError, "a closed question cannot be chosen twice")
}

func TestChooseQuestionOption_DeclineOptionClosesWithoutWriting(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{genPenalPath}, "Faut-il déclarer l'escroquerie ?",
		genAddOptions(genPenalPath, "NSP07"))
	version := f.Repo.Declarations[declId].Version
	docBefore := append([]byte(nil), f.Repo.Declarations[declId].Document...)

	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, version, 1)
	require.NoError(t, err)

	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status, "answered, not dismissed")
	assert.True(t, bytes.Equal(docBefore, f.Repo.Declarations[declId].Document), "nothing written")
	assert.Equal(t, version, f.Repo.Declarations[declId].Version, "the version is not bumped")
	assert.Equal(t, version, st.Declaration.Version)
	assert.Empty(t, st.Doc.Items["context.natureSoupcon.natureSoupconPenale"])
}

func TestChooseQuestionOption_AddOptionBeyondMaxItemsRefused(t *testing.T) {
	uc, f, declId := genReady(t, false)
	var fills []core.Fill
	for i, k := range []string{"a", "b", "c", "d", "e"} {
		fills = append(fills, core.Fill{Path: core.Path("context.natureSoupcon.cadreLegal[+" + k + "]"),
			Value: []string{"CL01", "CL02", "CL03", "CL04", "CL05"}[i]})
	}
	_, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version, fills)
	require.NoError(t, err)

	path := core.Path("context.natureSoupcon.cadreLegal[+z]")
	q := f.SeedQuestion(t, declId, []core.Path{path}, "Ajouter un cadre légal ?", genAddOptions(path, "CNV1"))
	docBefore := append([]byte(nil), f.Repo.Declarations[declId].Document...)
	version := f.Repo.Declarations[declId].Version

	_, err = uc.ChooseQuestionOption(f.Ctx, q.Id, version, 0)
	require.ErrorIs(t, err, models.BadParameterError)
	assert.Contains(t, err.Error(), "nombre maximal d'éléments atteint")
	assert.True(t, bytes.Equal(docBefore, f.Repo.Declarations[declId].Document), "nothing applied")
	assert.Equal(t, version, f.Repo.Declarations[declId].Version)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
}

func TestChooseQuestionOption_MultiFillAddOptionCreatesOneItem(t *testing.T) {
	uc, f, declId := genReady(t, false)
	genEditDoc(t, f, declId, func(doc *core.Document) { doc.AddItem("personnes.pp", "pp") })
	addr := core.Path("personnes.pp[pp-1].coordonnees.autreAdresse[+a]")
	fills := []core.Fill{
		{Path: addr + ".pays", Value: "FR"},
		{Path: addr + ".ville", Value: "Lyon"},
		{Path: addr + ".nomVoie", Value: "des Lilas"},
	}
	q := f.SeedQuestion(t, declId, []core.Path{addr + ".pays", addr + ".ville", addr + ".nomVoie"}, "Ajouter l'autre adresse ?",
		[]core.QuestionOption{{Label: "Ajouter", Fills: fills}, {Label: "Non"}})

	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, 0)
	require.NoError(t, err)

	keys := st.Doc.Items["personnes.pp[pp-1].coordonnees.autreAdresse"]
	require.Len(t, keys, 1, "one item holds all the fills")
	item := "personnes.pp[pp-1].coordonnees.autreAdresse[" + keys[0] + "]"
	for suffix, want := range map[string]string{".pays": "FR", ".ville": "Lyon", ".nomVoie": "des Lilas"} {
		v := st.Doc.Values[core.Path(item+suffix)]
		assert.Equal(t, want, v.Value, suffix)
		assert.Equal(t, core.OriginAnalyst, v.Origin)
		assert.True(t, v.Reviewed)
	}
	// A question that names the item prefix works as well.
	q2 := f.SeedQuestion(t, declId, []core.Path{addr}, "Encore ?", []core.QuestionOption{{Label: "Ajouter", Fills: []core.Fill{
		{Path: addr + ".pays", Value: "PT"}, {Path: addr + ".ville", Value: "Lisbonne"}}}, {Label: "Non"}})
	st, err = uc.ChooseQuestionOption(f.Ctx, q2.Id, st.Declaration.Version, 0)
	require.NoError(t, err)
	assert.Len(t, st.Doc.Items["personnes.pp[pp-1].coordonnees.autreAdresse"], 2)
}

func TestChooseQuestionOption_FillOutsideTheQuestionPathsRefusedOnAddQuestion(t *testing.T) {
	uc, f, declId := genReady(t, false)
	genEditDoc(t, f, declId, func(doc *core.Document) { doc.AddItem("personnes.pp", "pp") })
	addr := core.Path("personnes.pp[pp-1].coordonnees.autreAdresse[+a]")
	docBefore := append([]byte(nil), f.Repo.Declarations[declId].Document...)
	q := f.SeedQuestion(t, declId, []core.Path{addr + ".ville"}, "Ajouter ?", []core.QuestionOption{
		{Label: "Ajouter", Fills: []core.Fill{{Path: addr + ".ville", Value: "Lyon"}, {Path: addr + ".nomVoie", Value: "x"}}}, {Label: "Non"}})
	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, 0)
	require.ErrorIs(t, err, models.BadParameterError)
	assert.True(t, bytes.Equal(docBefore, f.Repo.Declarations[declId].Document))
}

func TestChooseQuestionOption_AddOptionAlreadyPresentClosesWithoutWriting(t *testing.T) {
	uc, f, declId := genReady(t, false)
	_, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
		[]core.Fill{{Path: "context.natureSoupcon.cadreLegal[+c]", Value: "CL02"},
			{Path: "context.natureSoupcon.natureSoupconPenale[+n]", Value: "NSP07"}})
	require.NoError(t, err)
	q := f.SeedQuestion(t, declId, []core.Path{genPenalPath}, "Déclarer ?", genAddOptions(genPenalPath, "NSP07"))
	version := f.Repo.Declarations[declId].Version
	docBefore := append([]byte(nil), f.Repo.Declarations[declId].Document...)

	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, version, 0)
	require.NoError(t, err)
	assert.Len(t, st.Doc.Items["context.natureSoupcon.natureSoupconPenale"], 1, "no duplicate")
	assert.True(t, bytes.Equal(docBefore, f.Repo.Declarations[declId].Document))
	assert.Equal(t, version, f.Repo.Declarations[declId].Version)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
}

// ---- Decline path checks ----

func genDeclineQuestion(t *testing.T, f *fakes, declId uuid.UUID) models.SarQuestion {
	t.Helper()
	return f.SeedQuestion(t, declId, []core.Path{genPenalPath}, "Déclarer ?", genAddOptions(genPenalPath, "NSP07"))
}

func TestDeclineQuestion_StaleVersionConflict(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := genDeclineQuestion(t, f, declId)
	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version-1, 1)
	require.ErrorIs(t, err, models.ErrSarVersionConflict)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
}

func TestDeclineQuestion_RefusedDuringJob(t *testing.T) {
	uc, f, declId := genReady(t, false)
	f.SeedJob(t, declId, models.SarJobKindReview, models.SarJobStatusRunning)
	q := genDeclineQuestion(t, f, declId)
	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, 1)
	require.ErrorIs(t, err, ErrGenerationInProgress)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
}

func TestDeclineQuestion_AlreadyClosedConflict(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := genDeclineQuestion(t, f, declId)
	version := f.Repo.Declarations[declId].Version
	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, version, 1)
	require.NoError(t, err)
	_, err = uc.ChooseQuestionOption(f.Ctx, q.Id, version, 1)
	require.ErrorIs(t, err, models.ConflictError)
}

func TestDeclineQuestion_OtherOrgForbidden(t *testing.T) {
	uc, f := newTestUsecase(t)
	foreign := models.SarDeclaration{
		Id: uuid.New(), OrganizationId: uuid.New(), CaseId: f.CaseId, FormatKey: tracfinFormatKey,
		ReportType: tracfinReportType, Version: 1,
		Document: []byte(`{"values":{},"items":{},"item_sources":{},"next_key":{}}`),
	}
	require.NoError(t, f.Repo.CreateSarDeclaration(f.Ctx, nil, foreign))
	q := genDeclineQuestion(t, f, foreign.Id)
	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, 1, 1)
	require.ErrorIs(t, err, models.ForbiddenError)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
}

func TestDismissQuestion_RefusedDuringJob(t *testing.T) {
	uc, f, declId := genReady(t, false)
	finding := f.SeedReviewFinding(t, declId, []core.Path{"organisme.libelle"}, "important", "Libellé à vérifier")
	f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusRunning)

	err := uc.DismissQuestion(f.Ctx, finding.Id)
	require.ErrorIs(t, err, ErrGenerationInProgress)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[finding.Id].Status)
}

// Choosing an option also closes the other open questions whose paths are all
// filled by it, like a manual edit; a question that adds a list item is not
// closed that way.
func TestChooseQuestionOption_ClosesTheOtherQuestionsOnTheWrittenPaths(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{"context.information.activiteLPS"}, "LPS ?", genOptionWithFill())
	other := f.SeedQuestion(t, declId, []core.Path{"context.information.activiteLPS"}, "Valeur non établie ?", nil)
	unrelated := f.SeedQuestion(t, declId, []core.Path{"organisme.libelle"}, "Libellé ?", nil)

	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, 0)
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[other.Id].Status,
		"another open question on the written path is answered too")
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[unrelated.Id].Status)
}
