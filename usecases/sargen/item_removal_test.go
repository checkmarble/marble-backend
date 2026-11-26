package sargen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Removing a person by hand rewrites what refers to it, like the removal option.
func TestRemoveItem_RewritesTheReferencesToTheRemovedPerson(t *testing.T) {
	uc, f, d, q := removalScenario(t)

	before := decodeDoc(t, d.Document)
	require.Len(t, before.ItemKeys("work.transactions"), 2)
	for _, k := range before.ItemKeys("work.transactions") {
		tx := core.Path("work.transactions[" + k + "]")
		require.Equal(t, string(removalPerson), before.Values[tx.Child("emetteurPersonne")].Value)
		require.Contains(t, before.Values[tx.Child("refEmission")].Value, string(removalPerson)+".")
	}

	st, err := uc.RemoveItem(f.Ctx, d.Id, d.Version, removalPerson)
	require.NoError(t, err)

	assert.Equal(t, d.Version+1, st.Declaration.Version)
	assert.Empty(t, st.Doc.ItemKeys("personnes.pp"), "the person is removed")
	require.Len(t, st.Doc.ItemKeys("work.transactions"), 2)
	for _, k := range st.Doc.ItemKeys("work.transactions") {
		tx := core.Path("work.transactions[" + k + "]")
		assert.Empty(t, st.Doc.Values[tx.Child("emetteurPersonne")].Value)
		free := st.Doc.Values[tx.Child("emetteurLibre")]
		assert.Equal(t, "Jean Zorg", free.Value)
		assert.Equal(t, core.OriginManual, free.Origin)
		assert.True(t, free.Reviewed)
		ref := st.Doc.Values[tx.Child("refEmission")]
		assert.Equal(t, "FR1420041010050500013M02606", ref.Value)
		assert.Equal(t, core.OriginManual, ref.Origin)
		assert.True(t, ref.Reviewed)
	}
	lines := st.Doc.ItemKeys("operations.operation")
	require.Len(t, lines, 1, "the operation lines are recomputed")
	line := core.Path("operations.operation[" + lines[0] + "]")
	assert.Equal(t, "Jean Zorg", st.Doc.Values[line.Child("emetteurTypeEmetteur")].Value)
	assert.Equal(t, "FR", st.Doc.Values[line.Child("paysOrigineFonds")].Value)
	for _, is := range st.Issues {
		assert.NotContains(t, []string{core.CodeDanglingRef, core.CodeBadRef}, is.Code, "%s: %s", is.Path, is.Message)
	}
	assert.Equal(t, models.SarQuestionStatusDismissed, f.Gen.Questions[q.Id].Status)
}

// The removal by hand and the removal through the question option give the same
// document.
func TestRemoveItem_SameDocumentAsTheRemovalQuestion(t *testing.T) {
	type cell struct {
		Value    string
		Origin   core.Origin
		Reviewed bool
	}
	cells := func(doc *core.Document) map[core.Path]cell {
		out := map[core.Path]cell{}
		for p, v := range doc.Values {
			if p == "context.general.refInterne" {
				continue // a random reference, different in every declaration
			}
			out[p] = cell{v.Value, v.Origin, v.Reviewed}
		}
		return out
	}

	ucQ, fQ, dQ, qQ := removalScenario(t)
	inside := fQ.SeedQuestion(t, dQ.Id, []core.Path{removalPerson.Child("etatCivil").Child("nomNaissance")}, "Nom ?", nil)
	outside := fQ.SeedQuestion(t, dQ.Id, []core.Path{"organisme.libelle"}, "Libellé ?", nil)
	stQ, err := ucQ.ChooseQuestionOption(fQ.Ctx, qQ.Id, dQ.Version, 1)
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusResolved, fQ.Gen.Questions[qQ.Id].Status)
	assert.Equal(t, models.SarQuestionStatusDismissed, fQ.Gen.Questions[inside.Id].Status)
	assert.Equal(t, models.SarQuestionStatusOpen, fQ.Gen.Questions[outside.Id].Status)

	ucH, fH, dH, qH := removalScenario(t)
	stH, err := ucH.RemoveItem(fH.Ctx, dH.Id, dH.Version, removalPerson)
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusDismissed, fH.Gen.Questions[qH.Id].Status)

	assert.Equal(t, cells(stQ.Doc), cells(stH.Doc))
}
