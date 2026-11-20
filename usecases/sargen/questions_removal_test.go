package sargen

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// A question whose option removes an item (a context person kept for its direct
// involvement), blocking until answered.

const removalPerson core.Path = "personnes.pp[pp-1]"

// removalScenario seeds, on top of a freshly generated declaration, a declared
// person with a name and an IBAN, two transactions that cite it as emitter and
// its IBAN as the emitting reference, and the blocking question of the placement
// (declare, or do not declare).
func removalScenario(t *testing.T) (*Usecase, *fakes, models.SarDeclaration, models.SarQuestion) {
	t.Helper()
	uc, f, declId := genReady(t, false)
	genEditDoc(t, f, declId, func(doc *core.Document) {
		person := doc.AddItem("personnes.pp", "pp")
		set := func(p core.Path, v string) { doc.Values[p] = core.Value{Value: v, Origin: core.OriginLLM} }
		set(person.Child("etatCivil").Child("nomNaissance"), "Zorg")
		set(doc.AddItem(person.Child("etatCivil").Child("prenom"), "prenom"), "Jean")
		iban := doc.AddItem(person.Child("referencesFinancieres").Child("referenceFinanciereIban"), "referenceFinanciereIban")
		set(iban.Child("iban"), "FR1420041010050500013M02606")
		for i := 0; i < 2; i++ {
			tx := doc.AddItem("work.transactions", "transactions")
			set(tx.Child("montant"), "14000")
			set(tx.Child("date"), "2026-03-01")
			set(tx.Child("devise"), "EUR")
			set(tx.Child("typeOperation"), "OT10")
			set(tx.Child("emetteurPersonne"), string(person))
			set(tx.Child("refEmission"), string(iban))
			set(tx.Child("beneficiaireLibre"), "Fournisseur PT")
			set(tx.Child("paysArrivee"), "PT")
		}
	})
	opts := []core.QuestionOption{
		{Label: "Déclarer persons/p-A comme personne impliquée"},
		{Label: "Ne pas déclarer", RemoveItem: removalPerson},
	}
	raw, err := json.Marshal(opts)
	require.NoError(t, err)
	q := f.SeedQuestion(t, declId, []core.Path{removalPerson}, "persons/p-A a été ajouté automatiquement", opts)
	q.Severity, q.Options = models.SarQuestionSeverityBlocking, raw
	f.Gen.Questions[q.Id] = q
	return uc, f, f.Repo.Declarations[declId], q
}

func TestChooseQuestionOption_DeclineRemovalRewritesAndRemoves(t *testing.T) {
	uc, f, d, q := removalScenario(t)

	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, d.Version, 1)
	require.NoError(t, err)

	assert.Equal(t, d.Version+1, st.Declaration.Version)
	assert.Empty(t, st.Doc.Items["personnes.pp"], "the person is removed")
	for _, k := range st.Doc.Items["work.transactions"] {
		tx := core.Path("work.transactions[" + k + "]")
		assert.Empty(t, st.Doc.Values[tx.Child("emetteurPersonne")].Value)
		assert.Equal(t, "Jean Zorg", st.Doc.Values[tx.Child("emetteurLibre")].Value)
		assert.Equal(t, "FR1420041010050500013M02606", st.Doc.Values[tx.Child("refEmission")].Value)
		assert.Equal(t, core.OriginAnalyst, st.Doc.Values[tx.Child("emetteurLibre")].Origin)
		assert.True(t, st.Doc.Values[tx.Child("emetteurLibre")].Reviewed)
	}
	lines := st.Doc.Items["operations.operation"]
	require.Len(t, lines, 1, "the operation lines are recomputed")
	line := core.Path("operations.operation[" + lines[0] + "]")
	assert.Equal(t, "Jean Zorg", st.Doc.Values[line.Child("emetteurTypeEmetteur")].Value)
	assert.Equal(t, "FR", st.Doc.Values[line.Child("paysOrigineFonds")].Value)
	for _, is := range st.Issues {
		assert.NotContains(t, []string{core.CodeDanglingRef, core.CodeBadRef}, is.Code, "%s: %s", is.Path, is.Message)
	}
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
}

func TestChooseQuestionOption_KeepingTheRemovableItemWritesNothing(t *testing.T) {
	uc, f, d, q := removalScenario(t)
	docBefore := append([]byte(nil), d.Document...)

	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, d.Version, 0)
	require.NoError(t, err)

	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status, "answered")
	assert.Equal(t, docBefore, f.Repo.Declarations[d.Id].Document, "nothing written")
	assert.Equal(t, d.Version, st.Declaration.Version, "the version is not bumped")
	assert.Len(t, st.Doc.Items["personnes.pp"], 1)
}

func TestChooseQuestionOption_RemovalWithAStaleVersionIsAConflict(t *testing.T) {
	uc, f, d, q := removalScenario(t)

	_, err := uc.ChooseQuestionOption(f.Ctx, q.Id, d.Version-1, 1)
	require.ErrorIs(t, err, models.ErrSarVersionConflict)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
	assert.Len(t, decodeDoc(t, f.Repo.Declarations[d.Id].Document).Items["personnes.pp"], 1)
}

func TestChooseQuestionOption_RemovalOfAnItemAlreadyGoneOnlyClosesTheQuestion(t *testing.T) {
	uc, f, d, q := removalScenario(t)
	genEditDoc(t, f, d.Id, func(doc *core.Document) { doc.RemoveItem(removalPerson) })
	d = f.Repo.Declarations[d.Id]

	st, err := uc.ChooseQuestionOption(f.Ctx, q.Id, d.Version, 1)
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
	assert.Equal(t, d.Version, st.Declaration.Version)
}

func TestChooseQuestionOption_RemovalOutsideTheQuestionPathsIsRefused(t *testing.T) {
	uc, f, d, q := removalScenario(t)
	forged := []core.QuestionOption{{Label: "Retirer l'autre", RemoveItem: "personnes.pp[pp-9]"}, {Label: "Garder"}}
	raw, err := json.Marshal(forged)
	require.NoError(t, err)
	q.Options = raw
	f.Gen.Questions[q.Id] = q

	_, err = uc.ChooseQuestionOption(f.Ctx, q.Id, d.Version, 0)
	require.ErrorIs(t, err, models.BadParameterError)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
}

// A blocking question refuses the export until it is answered; the todo lists it
// as blocking.
func TestExport_RefusedWhileABlockingQuestionIsOpen(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	q := f.SeedQuestion(t, d.Id, []core.Path{"personnes.pp[pp-1]"}, "Déclarer la personne ?", []core.QuestionOption{{Label: "Déclarer"}, {Label: "Ne pas déclarer", RemoveItem: "personnes.pp[pp-1]"}})
	q.Severity = models.SarQuestionSeverityBlocking
	f.Gen.Questions[q.Id] = q

	st, err := uc.GetDeclaration(f.Ctx, d.Id)
	require.NoError(t, err)
	item, ok := findTodo(st.Todo, todoRefIs(TodoQuestion, q.Id))
	require.True(t, ok)
	assert.Equal(t, TodoBlocking, item.Severity)

	_, err = uc.Export(f.Ctx, d.Id, d.Version)
	require.ErrorIs(t, err, models.BadParameterError)
	var ie *IssuesError
	require.ErrorAs(t, err, &ie)
	assert.Equal(t, models.SarCodeExportBlocked, ie.SarCode())
	var found bool
	for _, is := range ie.SarIssues() {
		if is.Code == CodeQuestionOpen {
			found = true
			assert.Equal(t, core.Path("personnes.pp[pp-1]"), is.Path)
			assert.Equal(t, "Déclarer la personne ?", is.Message)
		}
	}
	assert.True(t, found, "the open question is listed as a blocking issue")
	require.Empty(t, f.Blobs.Keys())

	// Answered (the person is declared): the export goes through.
	require.NoError(t, f.Gen.CloseSarQuestions(f.Ctx, nil, []uuid.UUID{q.Id}, models.SarQuestionStatusResolved))
	_, err = uc.Export(f.Ctx, d.Id, d.Version)
	require.NoError(t, err)
}

// An item removed by hand leaves nothing to answer: its questions are dismissed.
func TestRemoveItem_DismissesTheQuestionsAboutTheItem(t *testing.T) {
	uc, f, d, q := removalScenario(t)
	other := f.SeedQuestion(t, d.Id, []core.Path{"organisme.libelle"}, "Libellé ?", nil)

	_, err := uc.RemoveItem(f.Ctx, d.Id, d.Version, removalPerson)
	require.NoError(t, err)

	assert.Equal(t, models.SarQuestionStatusDismissed, f.Gen.Questions[q.Id].Status)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[other.Id].Status, "other questions are untouched")
}

// An empty required country gets a question whose option fills XX; choosing it
// removes the blocking issue.
func TestChooseQuestionOption_UnknownCountryOptionUnblocksTheExport(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	const path = "personnes.pm[pm-1].activiteCoordonnee.paysAutreTypeRegistre"
	genEditDoc(t, f, d.Id, func(doc *core.Document) {
		pm := doc.AddItem("personnes.pm", "pm")
		doc.Values[pm.Child("implication")] = core.Value{Value: "Personne soupçonnée / Personne impliquée", Origin: core.OriginLLM}
		doc.Values[pm.Child("activiteCoordonnee").Child("numeroIdentification").Child("type")] = core.Value{Value: "RN6", Origin: core.OriginLLM}
	})
	d = f.Repo.Declarations[d.Id]
	st, err := uc.GetDeclaration(f.Ctx, d.Id)
	require.NoError(t, err)
	var blocked bool
	for _, is := range st.Issues {
		blocked = blocked || (string(is.Path) == path && is.Blocks())
	}
	require.True(t, blocked, "the empty register country blocks the export")

	q := f.SeedQuestion(t, d.Id, []core.Path{path}, "valeur manquante", []core.QuestionOption{{
		Label: "Inconnu", Fills: []core.Fill{{Path: path, Value: "XX", Origin: core.OriginAnalyst}},
	}})
	st, err = uc.ChooseQuestionOption(f.Ctx, q.Id, d.Version, 0)
	require.NoError(t, err)
	assert.Equal(t, "XX", st.Doc.Values[path].Value)
	assert.Equal(t, core.OriginAnalyst, st.Doc.Values[path].Origin)
	for _, is := range st.Issues {
		assert.False(t, string(is.Path) == path && is.Blocks(), "the country no longer blocks: %s", is.Message)
	}
}

// A blocking question is never ignored: only one of its options closes it.
func TestDismissQuestion_BlockingQuestionIsRefused(t *testing.T) {
	uc, f, _, q := removalScenario(t)

	err := uc.DismissQuestion(f.Ctx, q.Id)
	var se *models.SarError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, models.SarCodeQuestionBlocking, se.Code)
	assert.ErrorIs(t, err, models.ConflictError)
	assert.Equal(t, "cette question doit être tranchée par une de ses options", se.Error())
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)

	// A question that is not blocking can still be dismissed.
	other := f.SeedQuestion(t, q.DeclarationId, []core.Path{"organisme.libelle"}, "Libellé ?", nil)
	require.NoError(t, uc.DismissQuestion(f.Ctx, other.Id))
}

// Defence in depth: the stored removal must name a list item the analyst could
// remove by hand, never a computed container or a field.
func TestChooseQuestionOption_RemovalOfANonListOrComputedItemIsRefused(t *testing.T) {
	for name, item := range map[string]core.Path{
		"a field":              "personnes.pp[pp-1].etatCivil.nomNaissance",
		"a computed list item": "operations.operation[operation-1]",
	} {
		t.Run(name, func(t *testing.T) {
			uc, f, d, q := removalScenario(t)
			genEditDoc(t, f, d.Id, func(doc *core.Document) { doc.AddItem("operations.operation", "operation") })
			d = f.Repo.Declarations[d.Id]
			raw, err := json.Marshal([]core.QuestionOption{{Label: "Garder"}, {Label: "Retirer", RemoveItem: item}})
			require.NoError(t, err)
			q.Paths, q.Options = []string{string(item)}, raw
			f.Gen.Questions[q.Id] = q

			_, err = uc.ChooseQuestionOption(f.Ctx, q.Id, d.Version, 1)
			var se *models.SarError
			require.ErrorAs(t, err, &se)
			assert.Equal(t, models.SarCodeInvalidRequest, se.Code)
			assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
		})
	}
}

type conflictingSelectionRepo struct{ SarSelectionRepository }

func (conflictingSelectionRepo) AddSarSelection(context.Context, repositories.Executor, []models.SarSelectedObject) error {
	return errors.Wrap(models.ConflictError, "objet déjà sélectionné pour cette déclaration")
}

// A concurrent duplicate selection reaches the client with its own code and message.
func TestUpdateSelection_AlreadySelectedHasItsOwnCode(t *testing.T) {
	uc, f, declId := genReady(t, false)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t2", "amount": 1}, time.Now())
	uc.selectionRepo = conflictingSelectionRepo{uc.selectionRepo}

	_, err := uc.UpdateSelection(f.Ctx, declId, f.Repo.Declarations[declId].Version,
		[]SelectionAdd{{Ref: selRef("transactions", "t2")}}, nil)

	var se *models.SarError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, models.SarCodeAlreadySelected, se.Code)
	assert.Equal(t, "objet déjà sélectionné pour cette déclaration", se.Error())
	assert.ErrorIs(t, err, models.ConflictError)
}
