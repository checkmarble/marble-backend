package tracfin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// The amount at stake never adds debits and credits (TRACFIN focus sheet « Le
// montant en jeu »).

type amountNoter interface {
	ComputedNote(doc *core.Document, p core.Path) string
}

func stakeTx(doc *core.Document, montant, sens string) core.Path {
	tx := compBase(montant, "2026-03-01")
	tx.sens = sens
	return addCompTx(doc, tx)
}

func computeStake(t *testing.T, doc *core.Document) core.Format {
	t.Helper()
	f := compFormat(t)
	compApply(t, f, doc, f.Compute(doc))
	return f
}

func TestAmount_DebitsAndCreditsAreNotSummed(t *testing.T) {
	doc := core.NewDocument()
	stakeTx(doc, "100", "debit")
	stakeTx(doc, "30", "credit")
	stakeTx(doc, "20", "credit")
	computeStake(t, doc)

	assert.Equal(t, "100", doc.Values["work.montantDebits"].Value)
	assert.Equal(t, "50", doc.Values["work.montantCredits"].Value)
	assert.Equal(t, core.OriginComputed, doc.Values["work.montantCredits"].Origin)
	// Both directions, no chosen one: no default, never 150, the analyst is asked.
	_, has := doc.Values["operations.montantTotal"]
	assert.False(t, has, "the total is left empty")
}

func TestAmount_TheJudgedDirectionWins(t *testing.T) {
	doc := core.NewDocument()
	stakeTx(doc, "100", "debit")
	stakeTx(doc, "30", "credit")
	doc.Values["work.sensMontantTotal"] = core.Value{Value: "credit", Origin: core.OriginLLM}
	computeStake(t, doc)

	assert.Equal(t, "30", doc.Values["operations.montantTotal"].Value, "the chosen direction, though smaller")
	assert.Equal(t, "100", doc.Values["work.montantDebits"].Value, "both totals are still computed")
}

func TestAmount_OnlyOneDirectionNeedsNoQuestion(t *testing.T) {
	doc := core.NewDocument()
	stakeTx(doc, "10", "debit")
	stakeTx(doc, "70", "debit")
	f := computeStake(t, doc)
	assert.Equal(t, "80", doc.Values["operations.montantTotal"].Value)
	assert.Empty(t, f.(core.DocumentQuestioner).DocumentQuestions(doc), "it follows from the data")

	// A chosen direction with no transaction changes nothing: the only direction wins.
	doc.Values["work.sensMontantTotal"] = core.Value{Value: "credit", Origin: core.OriginLLM}
	computeStake(t, doc)
	assert.Equal(t, "80", doc.Values["operations.montantTotal"].Value)
	note := f.(amountNoter).ComputedNote(doc, "operations.montantTotal")
	assert.Contains(t, note, "Sens retenu : débit (seul sens des transactions comptées)")
}

func TestAmount_BothDirectionsWithoutAChoiceAsksABlockingQuestion(t *testing.T) {
	doc := core.NewDocument()
	stakeTx(doc, "1000.5", "debit")
	stakeTx(doc, "30", "credit")
	stakeTx(doc, "20", "credit")
	f := computeStake(t, doc)
	_, has := doc.Values["operations.montantTotal"]
	require.False(t, has)

	qs := f.(core.DocumentQuestioner).DocumentQuestions(doc)
	require.Len(t, qs, 1)
	assert.True(t, qs[0].Blocking)
	assert.Equal(t, []core.Path{"work.sensMontantTotal"}, qs[0].Paths)
	require.Len(t, qs[0].Options, 2)
	assert.Equal(t, "Crédits : 50,00\u00a0€ (2 transactions)", qs[0].Options[0].Label)
	assert.Equal(t, "Débits : 1\u202f000,50\u00a0€ (1 transaction)", qs[0].Options[1].Label)
	assert.Equal(t, "credit", qs[0].Options[0].Fills[0].Value)
	assert.Equal(t, "debit", qs[0].Options[1].Fills[0].Value)

	// Choosing an option (the analyst's write) recomputes the total.
	choice := qs[0].Options[1].Fills[0]
	doc.Values[choice.Path] = core.Value{Value: choice.Value, Origin: core.OriginManual}
	computeStake(t, doc)
	assert.Equal(t, "1000.5", doc.Values["operations.montantTotal"].Value)
	assert.Empty(t, f.(core.DocumentQuestioner).DocumentQuestions(doc), "answered: nothing to ask")
	note := f.(amountNoter).ComputedNote(doc, "operations.montantTotal")
	assert.Contains(t, note, "Sens retenu : débit (choisi par l'analyste)")
}

func TestAmount_AStaleComputedTotalGoesAwayWhenTheDirectionIsNotDetermined(t *testing.T) {
	doc := core.NewDocument()
	stakeTx(doc, "100", "debit")
	f := computeStake(t, doc)
	require.Equal(t, "100", doc.Values["operations.montantTotal"].Value)
	stakeTx(doc, "30", "credit")
	compApply(t, f, doc, f.Compute(doc))
	_, has := doc.Values["operations.montantTotal"]
	assert.False(t, has)

	// A total the analyst typed stays, and nothing is asked.
	doc.Values["operations.montantTotal"] = core.Value{Value: "999", Origin: core.OriginManual}
	compApply(t, f, doc, f.Compute(doc))
	assert.Equal(t, "999", doc.Values["operations.montantTotal"].Value)
	assert.Empty(t, f.(core.DocumentQuestioner).DocumentQuestions(doc))
}

func TestAmount_DirectionIsDerivedFromDeclaredPersons(t *testing.T) {
	doc := core.NewDocument()
	const person = "personnes.pp[pp-1]"
	const other = "personnes.pp[pp-2]"
	doc.AddItem("personnes.pp", "pp")
	doc.AddItem("personnes.pp", "pp")
	set := func(tx core.Path, name, v string) {
		doc.Values[tx.Child(name)] = core.Value{Value: v, Origin: core.OriginLLM}
	}
	mk := func(montant string) core.Path {
		tx := compBase(montant, "2026-03-01")
		tx.sens = ""
		return addCompTx(doc, tx)
	}
	debit, credit, both, neither := mk("10"), mk("20"), mk("30"), mk("40")
	set(debit, "emetteurPersonne", person)
	set(credit, "beneficiairePersonne", person)
	set(both, "emetteurPersonne", person)
	set(both, "beneficiairePersonne", other)
	computeStake(t, doc)

	assert.Equal(t, "debit", doc.Values[debit.Child("sens")].Value)
	assert.Equal(t, core.OriginComputed, doc.Values[debit.Child("sens")].Origin)
	assert.Equal(t, "credit", doc.Values[credit.Child("sens")].Value)
	assert.Empty(t, doc.Values[both.Child("sens")].Value, "two declared persons: unknown")
	assert.Empty(t, doc.Values[neither.Child("sens")].Value, "no declared person: unknown")
	assert.Equal(t, "10", doc.Values["work.montantDebits"].Value)
	assert.Equal(t, "20", doc.Values["work.montantCredits"].Value)
	_, has := doc.Values["operations.montantTotal"]
	assert.False(t, has, "both directions and no choice: the total is not defaulted")
	doc.Values["work.sensMontantTotal"] = core.Value{Value: "credit", Origin: core.OriginManual}
	computeStake(t, doc)
	assert.Equal(t, "20", doc.Values["operations.montantTotal"].Value, "the unknown directions count in neither total")

	// The derived direction follows the document while it stays computed.
	delete(doc.Values, debit.Child("emetteurPersonne"))
	computeStake(t, doc)
	assert.Empty(t, doc.Values[debit.Child("sens")].Value)
}

func TestAmount_UnknownDirectionsAreExcludedAndCountedInTheNote(t *testing.T) {
	doc := core.NewDocument()
	stakeTx(doc, "100", "debit")
	stakeTx(doc, "40", "credit")
	stakeTx(doc, "500", "")
	stakeTx(doc, "600", "")
	f := computeStake(t, doc)

	doc.Values["work.sensMontantTotal"] = core.Value{Value: "debit", Origin: core.OriginLLM}
	computeStake(t, doc)
	assert.Equal(t, "100", doc.Values["operations.montantTotal"].Value)
	note := f.(amountNoter).ComputedNote(doc, "operations.montantTotal")
	assert.Contains(t, note, "valeur calculée à confirmer")
	assert.Contains(t, note, "Crédits : 40,00\u00a0€")
	assert.Contains(t, note, "débits : 100,00\u00a0€")
	assert.Contains(t, note, "Sens retenu : débit (choisi à l'étape de jugement)")
	assert.Contains(t, note, "Transactions sans sens : 2")
}

func TestAmount_NoKnownDirectionLeavesTheTotalEmptyAndAsksWithoutOption(t *testing.T) {
	doc := core.NewDocument()
	stakeTx(doc, "100", "")
	stakeTx(doc, "40", "")
	f := computeStake(t, doc)

	_, has := doc.Values["operations.montantTotal"]
	assert.False(t, has, "the old sum of debits and credits is gone")
	_, hasCredits := doc.Values["work.montantCredits"]
	_, hasDebits := doc.Values["work.montantDebits"]
	assert.False(t, hasCredits || hasDebits, "no direction total without a known direction")
	qs := f.(core.DocumentQuestioner).DocumentQuestions(doc)
	require.Len(t, qs, 1)
	assert.True(t, qs[0].Blocking)
	assert.Equal(t, []core.Path{"operations.montantTotal"}, qs[0].Paths)
	assert.Empty(t, qs[0].Options)
	note := f.(amountNoter).ComputedNote(doc, "operations.montantTotal")
	assert.Contains(t, note, "n'est pas déterminé")
	assert.Contains(t, note, "Transactions sans sens : 2")
}

func TestAmount_ManualValuesAreNeverOverwritten(t *testing.T) {
	doc := core.NewDocument()
	const person = "personnes.pp[pp-1]"
	doc.AddItem("personnes.pp", "pp")
	tx := stakeTx(doc, "100", "credit") // manual credit
	doc.Values[tx.Child("emetteurPersonne")] = core.Value{Value: person, Origin: core.OriginLLM}
	stakeTx(doc, "10", "debit")
	doc.Values["operations.montantTotal"] = core.Value{Value: "999", Origin: core.OriginManual}
	f := compFormat(t)
	res := f.Compute(doc)
	core.ApplyFills(f.Spec(), doc, res.Fills, core.ApplyOptions{Overwrite: core.OverwriteNonManual, SkipMaxItems: true})

	assert.Equal(t, "credit", doc.Values[tx.Child("sens")].Value, "a manual sens wins over the derivation (emitter is a declared person)")
	assert.Equal(t, core.OriginManual, doc.Values[tx.Child("sens")].Origin)
	assert.Equal(t, "999", doc.Values["operations.montantTotal"].Value, "a manual total is kept")
	assert.Equal(t, "100", doc.Values["work.montantCredits"].Value)

	// A sens the analyst cleared by hand stays empty.
	cleared := stakeTx(doc, "5", "")
	doc.Values[cleared.Child("sens")] = core.Value{Value: "", Origin: core.OriginManual}
	doc.Values[cleared.Child("emetteurPersonne")] = core.Value{Value: person, Origin: core.OriginLLM}
	res = f.Compute(doc)
	core.ApplyFills(f.Spec(), doc, res.Fills, core.ApplyOptions{Overwrite: core.OverwriteNonManual, SkipMaxItems: true})
	assert.Empty(t, doc.Values[cleared.Child("sens")].Value)
}

func TestAmount_NoCountableTransactionRemovesTheComputedTotals(t *testing.T) {
	doc := core.NewDocument()
	stakeTx(doc, "100", "debit")
	f := computeStake(t, doc)
	require.Equal(t, "100", doc.Values["work.montantDebits"].Value)

	for _, k := range doc.ItemKeys("work.transactions") {
		doc.RemoveItem(core.Path("work.transactions[" + k + "]"))
	}
	compApply(t, f, doc, f.Compute(doc))
	_, ok := doc.Values["work.montantDebits"]
	assert.False(t, ok)
}

// The unknown country the model wrote yields to the IBAN of a declared emitter,
// but never when reviewed, typed by the analyst or manual.
func TestCompute_UnknownCountryOfTheModelYieldsToTheIBAN(t *testing.T) {
	build := func(origin core.Origin, reviewed bool, value string) (*core.Document, core.Path, core.Format) {
		doc := core.NewDocument()
		person := doc.AddItem("personnes.pp", "pp")
		iban := doc.AddItem(person.Child("referencesFinancieres").Child("referenceFinanciereIban"), "referenceFinanciereIban")
		doc.Values[iban.Child("iban")] = core.Value{Value: "FR1420041010050500013M02606", Origin: core.OriginLLM}
		require.True(t, doc.SetItemSources(iban, []core.ObjectRef{{Table: "accounts", ObjectID: "a-1"}}))
		tx := stakeTx(doc, "100", "debit")
		require.True(t, doc.SetItemSources(tx, []core.ObjectRef{{Table: "accounts", ObjectID: "a-1"}}))
		doc.Values[tx.Child("emetteurPersonne")] = core.Value{Value: string(person), Origin: core.OriginLLM}
		doc.Values[tx.Child("paysOrigine")] = core.Value{Value: value, Origin: origin, Reviewed: reviewed}
		return doc, tx, computeStake(t, doc)
	}

	doc, tx, _ := build(core.OriginLLM, false, "XX")
	assert.Equal(t, "FR", doc.Values[tx.Child("paysOrigine")].Value)
	assert.Equal(t, core.OriginComputed, doc.Values[tx.Child("paysOrigine")].Origin)

	for name, c := range map[string]struct {
		origin   core.Origin
		reviewed bool
	}{"reviewed": {core.OriginLLM, true}, "manual": {core.OriginManual, false}} {
		doc, tx, _ := build(c.origin, c.reviewed, "XX")
		assert.Equal(t, "XX", doc.Values[tx.Child("paysOrigine")].Value, name)
	}
	// Another country the model wrote is not touched either.
	doc, tx, _ = build(core.OriginLLM, false, "DE")
	assert.Equal(t, "DE", doc.Values[tx.Child("paysOrigine")].Value)
}

func TestAmount_TotalProvenanceCitesTheJudgedDirection(t *testing.T) {
	doc := core.NewDocument()
	stakeTx(doc, "100", "debit")
	stakeTx(doc, "30", "credit")
	doc.Values["work.sensMontantTotal"] = core.Value{Value: "credit", Origin: core.OriginLLM}
	computeStake(t, doc)
	assert.Contains(t, doc.Values["operations.montantTotal"].Computation.Inputs, core.Path("work.sensMontantTotal"))

	doc2 := core.NewDocument()
	stakeTx(doc2, "100", "debit")
	computeStake(t, doc2)
	assert.NotContains(t, doc2.Values["operations.montantTotal"].Computation.Inputs, core.Path("work.sensMontantTotal"))
}

func TestAmount_FrenchLabelsForLargeAmounts(t *testing.T) {
	doc := core.NewDocument()
	stakeTx(doc, "116800", "credit")
	stakeTx(doc, "1234567.8", "debit")
	f := computeStake(t, doc)
	qs := f.(core.DocumentQuestioner).DocumentQuestions(doc)
	require.Len(t, qs, 1)
	assert.Equal(t, "Crédits : 116\u202f800,00\u00a0€ (1 transaction)", qs[0].Options[0].Label)
	assert.Equal(t, "Débits : 1\u202f234\u202f567,80\u00a0€ (1 transaction)", qs[0].Options[1].Label)
}

// Every amount of the note is written the French way, as in the labels of the
// amount question.
func TestAmount_NoteWritesTheAmountsTheFrenchWay(t *testing.T) {
	doc := core.NewDocument()
	stakeTx(doc, "671364.89", "debit")
	stakeTx(doc, "1250", "credit")
	doc.Values["work.sensMontantTotal"] = core.Value{Value: "debit", Origin: core.OriginManual}
	f := computeStake(t, doc)
	note := f.(amountNoter).ComputedNote(doc, "operations.montantTotal")
	assert.Contains(t, note, "Crédits : 1\u202f250,00\u00a0€")
	assert.Contains(t, note, "débits : 671\u202f364,89\u00a0€")
	assert.NotContains(t, note, "EUR")
}
