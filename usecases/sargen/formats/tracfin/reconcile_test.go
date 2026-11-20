package tracfin_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const (
	frIBAN = "FR1420041010050500013M02606"
	deIBAN = "DE89370400440532013000"
)

// reconcileDoc declares a person (Dupont) holding frIBAN as a financial
// reference, and returns the person and the reference.
func reconcileDoc() (*core.Document, core.Path, core.Path) {
	doc := core.NewDocument()
	person := doc.AddItem("personnes.pp", "pp")
	doc.Values[person.Child("etatCivil").Child("nomNaissance")] = core.Value{Value: "Dupont", Origin: core.OriginLLM}
	ref := doc.AddItem(person.Child("referencesFinancieres").Child("referenceFinanciereIban"), "referenceFinanciereIban")
	doc.Values[ref.Child("iban")] = core.Value{Value: frIBAN, Origin: core.OriginLLM}
	return doc, person, ref
}

func freeTx(doc *core.Document, id, free, ref string) core.Path {
	tx := addCompTx(doc, compTx{montant: "10", date: "2026-03-01", sens: "", devise: "EUR", typeOperation: "OT10", emetteurLibre: free, refEmission: ref})
	doc.ItemSources[tx] = []core.ObjectRef{{Table: "transactions", ObjectID: id}}
	return tx
}

func questionsOf(t *testing.T, doc *core.Document) []core.QuestionDraft {
	t.Helper()
	return compFormat(t).(core.DocumentQuestioner).DocumentQuestions(doc)
}

func snapshot(t *testing.T, doc *core.Document) string {
	t.Helper()
	b, err := json.Marshal(doc)
	require.NoError(t, err)
	return string(b)
}

func reconciliationOnly(qs []core.QuestionDraft) []core.QuestionDraft {
	var out []core.QuestionDraft
	for _, q := range qs {
		if !q.Blocking {
			out = append(out, q)
		}
	}
	return out
}

func TestReconcile_AFreeIssuerOnADeclaredIBANGetsOneGroupedQuestionAndNothingIsWritten(t *testing.T) {
	doc, person, ref := reconcileDoc()
	tx1 := freeTx(doc, "t1", "ACME SARL", "fr14 2004 1010 0505 0001 3m02606")
	tx2 := freeTx(doc, "t2", "Acme", frIBAN)
	before := snapshot(t, doc)

	qs := reconciliationOnly(questionsOf(t, doc))
	require.Len(t, qs, 1, "one question per group, not per transaction")
	assert.Equal(t, before, snapshot(t, doc), "nothing is written")
	q := qs[0]
	assert.False(t, q.Blocking, "a warning: the transactions export with free text")
	assert.ElementsMatch(t, []core.Path{
		tx1.Child("emetteurPersonne"), tx1.Child("emetteurLibre"), tx1.Child("refEmission"),
		tx2.Child("emetteurPersonne"), tx2.Child("emetteurLibre"), tx2.Child("refEmission"),
	}, q.Paths)
	assert.Contains(t, q.Text, "transactions/t1, transactions/t2")
	assert.Contains(t, q.Text, frIBAN)
	assert.Contains(t, q.Text, "Dupont")
	assert.Contains(t, q.Text, "Rattacher l'émetteur à Dupont ?")

	require.Len(t, q.Options, 2)
	link, free := q.Options[0], q.Options[1]
	assert.Equal(t, "Rattacher à Dupont", link.Label)
	assert.Equal(t, "Laisser en texte libre", free.Label)
	assert.Empty(t, free.Fills)
	assert.Empty(t, free.Clear)
	values := map[core.Path]string{}
	for _, f := range link.Fills {
		values[f.Path] = f.Value
	}
	assert.Equal(t, string(person), values[tx1.Child("emetteurPersonne")])
	assert.Equal(t, string(ref), values[tx1.Child("refEmission")])
	assert.Equal(t, string(person), values[tx2.Child("emetteurPersonne")])
	assert.ElementsMatch(t, []core.Path{tx1.Child("emetteurLibre"), tx2.Child("emetteurLibre")}, link.Clear)
}

func TestReconcile_ChoosingTheLinkRelinksAllTheTransactionsAndTheOperationLinesRecompute(t *testing.T) {
	doc, person, ref := reconcileDoc()
	tx1 := freeTx(doc, "t1", "ACME SARL", frIBAN)
	tx2 := freeTx(doc, "t2", "Acme", frIBAN)
	f := compFormat(t)
	compApply(t, f, doc, f.Compute(doc))
	require.Len(t, doc.Items["operations.operation"], 2, "two lines while the issuers are different free texts")

	q := reconciliationOnly(f.(core.DocumentQuestioner).DocumentQuestions(doc))[0]
	opt := q.Options[0]
	for _, p := range opt.Clear {
		delete(doc.Values, p)
	}
	for _, fl := range opt.Fills {
		fl.Origin = core.OriginAnalyst
		res := core.ApplyFills(f.Spec(), doc, []core.Fill{fl}, core.ApplyOptions{Overwrite: core.OverwriteAll})
		require.Empty(t, res.Rejected)
	}
	compApply(t, f, doc, f.Compute(doc))

	for _, tx := range []core.Path{tx1, tx2} {
		assert.Equal(t, string(person), doc.Values[tx.Child("emetteurPersonne")].Value)
		assert.Equal(t, string(ref), doc.Values[tx.Child("refEmission")].Value)
		_, has := doc.Values[tx.Child("emetteurLibre")]
		assert.False(t, has, "the free-text name is cleared")
	}
	assert.Len(t, doc.Items["operations.operation"], 1, "the lines recompute: one issuer now")
	assert.Empty(t, reconciliationOnly(f.(core.DocumentQuestioner).DocumentQuestions(doc)), "nothing left to ask")
}

func TestReconcile_NoQuestionWithoutAFullValidSingleMatch(t *testing.T) {
	cases := map[string]func(doc *core.Document, person core.Path){
		"masked IBAN":      func(d *core.Document, _ core.Path) { freeTx(d, "t", "ACME", "FR14 2004 **** **** 2606") },
		"partial IBAN":     func(d *core.Document, _ core.Path) { freeTx(d, "t", "ACME", frIBAN[:12]) },
		"invalid checksum": func(d *core.Document, _ core.Path) { freeTx(d, "t", "ACME", "FR1420041010050500013M02607") },
		"not an IBAN":      func(d *core.Document, _ core.Path) { freeTx(d, "t", "ACME", "12345678901") },
		"another IBAN":     func(d *core.Document, _ core.Path) { freeTx(d, "t", "ACME", deIBAN) },
		"already linked": func(d *core.Document, p core.Path) {
			tx := freeTx(d, "t", "ACME", frIBAN)
			d.Values[tx.Child("emetteurPersonne")] = core.Value{Value: string(p), Origin: core.OriginLLM}
		},
		"no free name": func(d *core.Document, _ core.Path) { freeTx(d, "t", "", frIBAN) },
		"two persons hold the IBAN": func(d *core.Document, _ core.Path) {
			other := d.AddItem("personnes.pm", "pm")
			r := d.AddItem(other.Child("referencesFinancieres").Child("referenceFinanciereIban"), "referenceFinanciereIban")
			d.Values[r.Child("iban")] = core.Value{Value: frIBAN, Origin: core.OriginLLM}
			freeTx(d, "t", "ACME", frIBAN)
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			doc, person, _ := reconcileDoc()
			build(doc, person)
			assert.Empty(t, reconciliationOnly(questionsOf(t, doc)))
		})
	}
}

func TestReconcile_SidesAndIBANsFormTheirOwnGroups(t *testing.T) {
	doc, person, _ := reconcileDoc()
	r2 := doc.AddItem(person.Child("referencesFinancieres").Child("referenceFinanciereIban"), "referenceFinanciereIban")
	doc.Values[r2.Child("iban")] = core.Value{Value: deIBAN, Origin: core.OriginLLM}
	freeTx(doc, "t1", "ACME", frIBAN)
	freeTx(doc, "t2", "ACME", deIBAN)
	tx := addCompTx(doc, compTx{montant: "5", date: "2026-03-01", devise: "EUR", typeOperation: "OT10", beneficiaireLibre: "Dupont J", refReception: frIBAN})
	doc.ItemSources[tx] = []core.ObjectRef{{Table: "transactions", ObjectID: "t3"}}

	qs := reconciliationOnly(questionsOf(t, doc))
	require.Len(t, qs, 3)
	var beneficiary int
	for _, q := range qs {
		if len(q.Paths) > 0 && q.Paths[0] == tx.Child("beneficiairePersonne") {
			beneficiary++
			assert.Contains(t, q.Text, "Rattacher le bénéficiaire à Dupont ?")
		}
	}
	assert.Equal(t, 1, beneficiary)
}

// ---- code sources ----

func TestCodeSource_CountryListsAreFedByCountryAndIban(t *testing.T) {
	f := compFormat(t)
	m, ok := f.(core.CodeSourceMapper)
	require.True(t, ok)
	node := f.Spec().NodeAt("work.transactions[].paysOrigine")
	require.NotNil(t, node)
	assert.ElementsMatch(t, []models.FieldSemanticType{models.FieldSemanticTypeCountry, models.FieldSemanticTypeIban}, m.CodeListSemantics(node.CodeList))
	assert.Empty(t, m.CodeListSemantics("liste_ref_devises.csv"))

	list := node.CodeList
	for raw, want := range map[string]string{"HK": "HK", "hk": "HK", "Hong Kong": "HK", "hong-kong": "", "Atlantis": "", "XX": ""} {
		got, ok := m.CodeFromValue(list, models.FieldSemanticTypeCountry, raw)
		assert.Equal(t, want != "", ok, raw)
		assert.Equal(t, want, got, raw)
	}
	got, ok := m.CodeFromValue(list, models.FieldSemanticTypeIban, "fr14 2004 1010 0505 0001 3m02606")
	assert.True(t, ok)
	assert.Equal(t, "FR", got)
	_, ok = m.CodeFromValue(list, models.FieldSemanticTypeIban, "FR1420041010050500013M02607")
	assert.False(t, ok, "an invalid IBAN gives no country")
}
