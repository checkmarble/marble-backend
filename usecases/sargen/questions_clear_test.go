package sargen

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

func clearOptions() []core.QuestionOption {
	return []core.QuestionOption{
		{
			Label: "Remplacer",
			Fills: []core.Fill{{Path: "context.information.activiteLPS", Value: "true"}},
			Clear: []core.Path{"context.general.numeroProcedure"},
		},
		{Label: "Laisser"},
	}
}

func TestChooseQuestionOption_ClearDeletesThenFills(t *testing.T) {
	uc, f, declId := genReady(t, false)
	_, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
		[]core.Fill{{Path: "context.general.numeroProcedure", Value: "P-1"}})
	require.NoError(t, err)
	q := f.SeedQuestion(t, declId, []core.Path{"context.general.numeroProcedure", "context.information.activiteLPS"},
		"Remplacer ?", clearOptions())

	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, 0)
	require.NoError(t, err)
	_, has := st.Doc.Values["context.general.numeroProcedure"]
	assert.False(t, has, "the cleared value is deleted")
	v := st.Doc.Values["context.information.activiteLPS"]
	assert.Equal(t, "true", v.Value)
	assert.Equal(t, core.OriginAnalyst, v.Origin)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
}

func TestChooseQuestionOption_ClearOutsideTheQuestionRefused(t *testing.T) {
	uc, f, declId := genReady(t, false)
	_, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
		[]core.Fill{{Path: "context.general.numeroProcedure", Value: "P-1"}})
	require.NoError(t, err)
	q := f.SeedQuestion(t, declId, []core.Path{"context.information.activiteLPS"}, "Remplacer ?", clearOptions())
	docBefore := append([]byte(nil), f.Repo.Declarations[declId].Document...)

	_, err = uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, 0)
	require.ErrorIs(t, err, models.BadParameterError)
	assert.True(t, bytes.Equal(docBefore, f.Repo.Declarations[declId].Document), "nothing applied")
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
}

func TestChooseQuestionOption_ClearOfASettingRefused(t *testing.T) {
	uc, f, declId := genReady(t, false)
	opts := []core.QuestionOption{{Label: "Vider", Clear: []core.Path{"organisme.libelle"}}}
	q := f.SeedQuestion(t, declId, []core.Path{"organisme.libelle"}, "Vider ?", opts)
	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, 0)
	require.ErrorIs(t, err, models.BadParameterError)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
}

func TestChooseQuestionOption_OptionThatWritesNothingResolvesWhenASiblingWrites(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{"context.general.numeroProcedure", "context.information.activiteLPS"},
		"Remplacer ?", clearOptions())
	version := f.Repo.Declarations[declId].Version
	docBefore := append([]byte(nil), f.Repo.Declarations[declId].Document...)

	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, version, 1)
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
	assert.Equal(t, version, st.Declaration.Version, "nothing written, version kept")
	assert.True(t, bytes.Equal(docBefore, f.Repo.Declarations[declId].Document))
}

// A review finding carries options like a question: choosing one writes its fills.
func TestChooseQuestionOption_FindingOptionWritesItsFills(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{"context.information.activiteLPS"}, "LPS lisible dans les données", genOptionWithFill())
	q.Kind = models.SarQuestionKindReviewFinding
	q.Severity = "important"
	f.Gen.Questions[q.Id] = q

	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, 0)
	require.NoError(t, err)
	assert.Equal(t, "true", st.Doc.Values["context.information.activiteLPS"].Value)
	assert.Equal(t, core.OriginAnalyst, st.Doc.Values["context.information.activiteLPS"].Origin)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
}

// A finding that replaced a blocking question stays blocking: it refuses the
// export, is listed as blocking and cannot be dismissed.
func TestBlockingFindingKeepsTheExportClosed(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	q := f.SeedQuestion(t, d.Id, []core.Path{"personnes.pp[pp-1]"}, "Les données donnent la valeur", nil)
	q.Kind = models.SarQuestionKindReviewFinding
	q.Severity = models.SarQuestionSeverityBlocking
	f.Gen.Questions[q.Id] = q

	st, err := uc.GetDeclaration(f.Ctx, d.Id)
	require.NoError(t, err)
	item, ok := findTodo(st.Todo, todoRefIs(TodoReviewFinding, q.Id))
	require.True(t, ok)
	assert.Equal(t, TodoBlocking, item.Severity)

	_, err = uc.Export(f.Ctx, d.Id, d.Version)
	var ie *IssuesError
	require.ErrorAs(t, err, &ie)
	assert.Equal(t, models.SarCodeExportBlocked, ie.SarCode())

	err = uc.DismissQuestion(f.Ctx, q.Id)
	requireSarCode(t, err, models.SarCodeQuestionBlocking)
}

// The total of the operations is a computed value the analyst gives by hand when
// no direction is known: a free answer is accepted there, and nowhere else on a
// computed field.
func TestAnswerQuestion_TheTotalOfTheOperationsCanBeGivenByHand(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{"operations.montantTotal"}, "Indiquez le montant total", nil)
	st, err := uc.AnswerQuestion(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, "250", "")
	require.NoError(t, err)
	assert.Equal(t, "250", st.Doc.Values["operations.montantTotal"].Value)
	assert.Equal(t, core.OriginManual, st.Doc.Values["operations.montantTotal"].Origin)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
}

// Choosing the direction of the amount at stake writes work.sensMontantTotal and
// the total of the operations is recomputed in the same write.
func TestChooseQuestionOption_DirectionOfTheAmountRecomputesTheTotal(t *testing.T) {
	uc, f, declId := genReady(t, false)
	genEditDoc(t, f, declId, func(doc *core.Document) {
		for _, tx := range []struct{ montant, sens string }{{"100", "debit"}, {"30", "credit"}} {
			item := doc.AddItem("work.transactions", "transactions")
			for name, v := range map[string]string{"montant": tx.montant, "sens": tx.sens, "devise": "EUR", "date": "2026-03-01", "typeOperation": "OT10"} {
				doc.Values[item.Child(name)] = core.Value{Value: v, Origin: core.OriginManual}
			}
		}
	})
	q := f.SeedQuestion(t, declId, []core.Path{"work.sensMontantTotal"}, "Quel sens retenir ?", []core.QuestionOption{
		{Label: "Crédits : 30 € (1 transaction)", Fills: []core.Fill{{Path: "work.sensMontantTotal", Value: "credit"}}},
		{Label: "Débits : 100 € (1 transaction)", Fills: []core.Fill{{Path: "work.sensMontantTotal", Value: "debit"}}},
	})
	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, 1)
	require.NoError(t, err)
	assert.Equal(t, "debit", st.Doc.Values["work.sensMontantTotal"].Value)
	assert.Equal(t, "100", st.Doc.Values["operations.montantTotal"].Value)
}

// ---- a blocking finding that replaced a blocking question is always answerable ----

const findingField = core.Path("context.information.activiteLPS")

// seedBlockingFinding seeds, on a valid declaration, a blocking finding as a review
// leaves it when it contests a blocking question: its paths are the question's
// field and an element, and it has (or not) an option.
func seedBlockingFinding(t *testing.T, uc *Usecase, f *fakes, opts []core.QuestionOption) (models.SarDeclaration, models.SarQuestion) {
	t.Helper()
	d := seedValidDeclaration(t, uc, f)
	// The field is empty in the declaration: the finding stands for the missing value.
	doc := decodeDoc(t, d.Document)
	delete(doc.Values, findingField)
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	d.Document = raw
	f.Repo.Declarations[d.Id] = d
	q := f.SeedQuestion(t, d.Id, []core.Path{findingField, "personnes.pp[pp-1]"}, "Les données donnent la valeur", opts)
	q.Kind = models.SarQuestionKindReviewFinding
	q.Severity = models.SarQuestionSeverityBlocking
	f.Gen.Questions[q.Id] = q
	return d, q
}

func requireExportBlockedBy(t *testing.T, uc *Usecase, f *fakes, d models.SarDeclaration, message string) {
	t.Helper()
	_, err := uc.Export(f.Ctx, d.Id, f.Repo.Declarations[d.Id].Version)
	var ie *IssuesError
	require.ErrorAs(t, err, &ie)
	var found bool
	for _, is := range ie.SarIssues() {
		if is.Code == CodeQuestionOpen && is.Message == message {
			found = true
		}
	}
	assert.True(t, found, "the export says why it is blocked")
}

func TestBlockingFinding_AnswerThroughTheFieldReopensTheExport(t *testing.T) {
	uc, f := newTestUsecase(t)
	d, q := seedBlockingFinding(t, uc, f, nil)
	requireExportBlockedBy(t, uc, f, d, "Les données donnent la valeur")

	_, err := uc.AnswerQuestion(f.Ctx, q.Id, f.Repo.Declarations[d.Id].Version, "true", findingField)
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
	_, err = uc.Export(f.Ctx, d.Id, f.Repo.Declarations[d.Id].Version)
	require.NoError(t, err)
}

func TestBlockingFinding_ChoosingItsOptionReopensTheExport(t *testing.T) {
	uc, f := newTestUsecase(t)
	d, q := seedBlockingFinding(t, uc, f, []core.QuestionOption{{Label: "Oui", Fills: []core.Fill{{Path: findingField, Value: "true"}}}})
	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, f.Repo.Declarations[d.Id].Version, 0)
	require.NoError(t, err)
	_, err = uc.Export(f.Ctx, d.Id, f.Repo.Declarations[d.Id].Version)
	require.NoError(t, err)
}

// A manual edit of the field closes the finding too: nobody has to find /answer.
func TestBlockingFinding_ManualEditOfItsFieldClosesIt(t *testing.T) {
	uc, f := newTestUsecase(t)
	d, q := seedBlockingFinding(t, uc, f, nil)
	_, err := uc.UpdateFields(f.Ctx, d.Id, f.Repo.Declarations[d.Id].Version, []core.Fill{{Path: findingField, Value: "false"}})
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
	_, err = uc.Export(f.Ctx, d.Id, f.Repo.Declarations[d.Id].Version)
	require.NoError(t, err)
}

// An element path of the finding cannot be answered: only its field can.
func TestBlockingFinding_AnswerOnTheElementPathIsRefused(t *testing.T) {
	uc, f := newTestUsecase(t)
	d, q := seedBlockingFinding(t, uc, f, nil)
	_, err := uc.AnswerQuestion(f.Ctx, q.Id, f.Repo.Declarations[d.Id].Version, "x", "personnes.pp[pp-1]")
	requireSarCode(t, err, models.SarCodeInvalidRequest)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
}
