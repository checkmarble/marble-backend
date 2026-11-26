package tracfin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

func computedInputsMapper(t *testing.T, f core.Format) core.ComputedInputsMapper {
	t.Helper()
	m, ok := f.(core.ComputedInputsMapper)
	require.True(t, ok, "the TRACFIN format exposes the inputs of its computed fields")
	return m
}

// Two transactions that make one line: every attribute of the line maps to the
// field of both transactions, the amounts to both montants.
func TestComputedInputFields_LineOfTwoTransactions(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	tx := compBase("100.00", "2024-01-05")
	tx.emetteurLibre, tx.beneficiaireLibre = "ACME", "Bob"
	tx.refEmission, tx.refReception = "FR7630006000011234567890189", ""
	t1 := addCompTx(doc, tx)
	t2 := addCompTx(doc, tx)
	compApply(t, f, doc, f.Compute(doc))
	lines := compLines(doc)
	require.Len(t, lines, 1)
	line := lines[0]
	m := computedInputsMapper(t, f)

	assert.Equal(t, []core.Path{t1.Child("refReception"), t2.Child("refReception")},
		m.ComputedInputFields(doc, line.Child("refFinanciereReception")))
	assert.Equal(t, []core.Path{t1.Child("refEmission"), t2.Child("refEmission")},
		m.ComputedInputFields(doc, line.Child("refFinanciereEmission")))
	assert.Equal(t, []core.Path{t1.Child("paysArrivee"), t2.Child("paysArrivee")},
		m.ComputedInputFields(doc, line.Child("paysArriveeFonds")))
	assert.Equal(t, []core.Path{t1.Child("paysOrigine"), t2.Child("paysOrigine")},
		m.ComputedInputFields(doc, line.Child("paysOrigineFonds")))
	assert.Equal(t, []core.Path{t1.Child("typeOperation"), t2.Child("typeOperation")},
		m.ComputedInputFields(doc, line.Child("typeOperation")))
	assert.Equal(t, []core.Path{t1.Child("emetteurPersonne"), t1.Child("emetteurLibre"), t2.Child("emetteurPersonne"), t2.Child("emetteurLibre")},
		m.ComputedInputFields(doc, line.Child("emetteurTypeEmetteur")))
	assert.Equal(t, []core.Path{t1.Child("beneficiairePersonne"), t1.Child("beneficiaireLibre"), t2.Child("beneficiairePersonne"), t2.Child("beneficiaireLibre")},
		m.ComputedInputFields(doc, line.Child("beneficiaireTypeBeneficiaire")))
	assert.Equal(t, []core.Path{t1.Child("montant"), t2.Child("montant")}, m.ComputedInputFields(doc, line.Child("montantTotal")))
	assert.Equal(t, []core.Path{t1.Child("montant"), t2.Child("montant")}, m.ComputedInputFields(doc, line.Child("nombreOperations")))
}

// Other computed paths, unknown attributes and plain fields have no inputs.
func TestComputedInputFields_OtherPathsHaveNone(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	t1 := addCompTx(doc, compBase("100.00", "2024-01-05"))
	compApply(t, f, doc, f.Compute(doc))
	line := compLines(doc)[0]
	m := computedInputsMapper(t, f)
	for _, p := range []core.Path{
		"operations.dateDebut", "operations.dateFin", "operations.montantTotal", line, line.Child("inconnu"),
		t1.Child("refReception"), "work.montantCredits",
	} {
		assert.Empty(t, m.ComputedInputFields(doc, p), p)
	}
	assert.Empty(t, m.ComputedInputFields(doc, "operations.operation[nope].refFinanciereReception"), "a line that is not in the document")
}

// The line builder and the capability read the same table: each attribute a line
// carries has the value of one of the transaction fields the capability names.
func TestComputedInputFields_AgreeWithTheLineBuilder(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	tx := compBase("100.00", "2024-01-05")
	tx.emetteurLibre, tx.beneficiaireLibre = "ACME", "Bob"
	tx.refEmission, tx.refReception = "FR7630006000011234567890189", "DE89370400440532013000"
	addCompTx(doc, tx)
	compApply(t, f, doc, f.Compute(doc))
	line := compLines(doc)[0]
	m := computedInputsMapper(t, f)
	for _, attr := range []string{
		"typeOperation", "emetteurTypeEmetteur", "beneficiaireTypeBeneficiaire", "refFinanciereEmission",
		"refFinanciereReception", "paysOrigineFonds", "paysArriveeFonds",
	} {
		want := compLineValue(doc, line, attr)
		require.NotEmpty(t, want, attr)
		var got []string
		for _, in := range m.ComputedInputFields(doc, line.Child(attr)) {
			got = append(got, doc.Values[in].Value)
		}
		assert.Contains(t, got, want, attr)
	}
}
