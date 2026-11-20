package tracfin_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
)

// compTx is one transaction of work.transactions as the tests build it.
type compTx struct {
	montant, date, sens, devise, typeOperation string
	emetteurPersonne, emetteurLibre            string
	beneficiaireLibre                          string
	refEmission, refReception                  string
	paysOrigine, paysArrivee                   string
	autreTypeOperation                         string
}

// compBase is a valid EUR debit transaction, OT10 (virement) from FR to DE.
func compBase(montant, date string) compTx {
	return compTx{
		montant: montant, date: date, sens: "debit", devise: "EUR", typeOperation: "OT10",
		paysOrigine: "FR", paysArrivee: "DE",
	}
}

func compFormat(t *testing.T) core.Format {
	t.Helper()
	f, ok := tracfin.New(tracfin.CurrentVersion)
	require.True(t, ok)
	return f
}

// addCompTx appends a transaction to work.transactions and returns its path.
func addCompTx(doc *core.Document, tx compTx) core.Path {
	item := doc.AddItem("work.transactions", "transactions")
	set := func(name, v string) {
		if v != "" {
			doc.Values[item.Child(name)] = core.Value{Value: v, Origin: core.OriginManual}
		}
	}
	set("montant", tx.montant)
	set("date", tx.date)
	set("sens", tx.sens)
	set("devise", tx.devise)
	set("typeOperation", tx.typeOperation)
	set("autretypeOperation", tx.autreTypeOperation)
	set("emetteurPersonne", tx.emetteurPersonne)
	set("emetteurLibre", tx.emetteurLibre)
	set("beneficiaireLibre", tx.beneficiaireLibre)
	set("refEmission", tx.refEmission)
	set("refReception", tx.refReception)
	set("paysOrigine", tx.paysOrigine)
	set("paysArrivee", tx.paysArrivee)
	return item
}

// compApply applies a Compute result the way the orchestrator does: the fills
// with the non-manual policy and no item cap (the validator reports excess),
// then the removals.
func compApply(t *testing.T, f core.Format, doc *core.Document, res core.ComputeResult) core.ApplyResult {
	t.Helper()
	ar := core.ApplyFills(f.Spec(), doc, res.Fills, core.ApplyOptions{
		Overwrite: core.OverwriteNonManual, SkipMaxItems: true,
	})
	require.Empty(t, ar.Rejected, "computed fills must all be accepted")
	for _, p := range res.RemoveItems {
		doc.RemoveItem(p)
	}
	return ar
}

// compLines returns the operation lines of the document, in item order.
func compLines(doc *core.Document) []core.Path {
	out := make([]core.Path, 0, len(doc.Items["operations.operation"]))
	for _, k := range doc.Items["operations.operation"] {
		out = append(out, core.Path("operations.operation["+k+"]"))
	}
	return out
}

func compLineValue(doc *core.Document, line core.Path, field string) string {
	return doc.Values[line.Child(field)].Value
}

// compFindLine returns the only line whose field holds v.
func compFindLine(t *testing.T, doc *core.Document, field, v string) core.Path {
	t.Helper()
	var found []core.Path
	for _, l := range compLines(doc) {
		if compLineValue(doc, l, field) == v {
			found = append(found, l)
		}
	}
	require.Len(t, found, 1, "exactly one operation line with %s = %q", field, v)
	return found[0]
}

func compDecimal(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	require.NoError(t, err, "decimal value %q", s)
	return v
}

func requireCompDecimal(t *testing.T, want float64, got string) {
	t.Helper()
	assert.InDelta(t, want, compDecimal(t, got), 0.001)
}

func TestCompute_GroupsTransactionsBySameLineAttributes(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	// Group A: three OT10 transactions with the same attributes.
	addCompTx(doc, compBase("100", "2026-03-01"))
	addCompTx(doc, compBase("200", "2026-03-02"))
	addCompTx(doc, compBase("300", "2026-03-03"))
	// Group B: two OT06 transactions.
	b1 := compBase("50", "2026-03-04")
	b1.typeOperation = "OT06"
	addCompTx(doc, b1)
	b2 := compBase("25", "2026-03-05")
	b2.typeOperation = "OT06"
	addCompTx(doc, b2)

	res := f.Compute(doc)
	compApply(t, f, doc, res)

	require.Len(t, compLines(doc), 2, "five transactions in two groups make two operations")
	a := compFindLine(t, doc, "typeOperation", "OT10")
	requireCompDecimal(t, 600, compLineValue(doc, a, "montantTotal"))
	requireCompDecimal(t, 3, compLineValue(doc, a, "nombreOperations"))
	b := compFindLine(t, doc, "typeOperation", "OT06")
	requireCompDecimal(t, 75, compLineValue(doc, b, "montantTotal"))
	requireCompDecimal(t, 2, compLineValue(doc, b, "nombreOperations"))
}

func TestCompute_PeriodAndTotal(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	addCompTx(doc, compBase("100", "2026-03-10"))
	addCompTx(doc, compBase("50", "2026-01-05"))
	addCompTx(doc, compBase("25", "2026-05-20"))

	res := f.Compute(doc)
	compApply(t, f, doc, res)

	assert.Equal(t, "2026-01-05", doc.Values["operations.dateDebut"].Value)
	assert.Equal(t, "2026-05-20", doc.Values["operations.dateFin"].Value)
	requireCompDecimal(t, 175, doc.Values["operations.montantTotal"].Value)

	// The total is computed but not reviewed: the analyst confirms it.
	var total *core.Fill
	for i := range res.Fills {
		if res.Fills[i].Path == "operations.montantTotal" {
			total = &res.Fills[i]
		}
	}
	require.NotNil(t, total, "the total is computed")
	assert.Equal(t, core.OriginComputed, total.Origin)
	assert.False(t, total.Reviewed, "the computed total must not be marked reviewed")
	assert.False(t, doc.Values["operations.montantTotal"].Reviewed)
}

func TestCompute_FreeTextCounterpartyGroupsSeparately(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	a := compBase("10", "2026-03-01")
	a.emetteurLibre = "Société A"
	addCompTx(doc, a)
	b := compBase("20", "2026-03-02")
	b.emetteurLibre = "Société B"
	addCompTx(doc, b)
	// Same free-text name as a: same group.
	a2 := compBase("5", "2026-03-03")
	a2.emetteurLibre = "Société A"
	addCompTx(doc, a2)
	// A referenced person (declared in the document: a reference to an absent
	// item is rejected by the writer) is never grouped with a free text.
	doc.AddItem("personnes.pp", "pp")
	ref := compBase("30", "2026-03-04")
	ref.emetteurPersonne = "personnes.pp[pp-1]"
	addCompTx(doc, ref)

	res := f.Compute(doc)
	compApply(t, f, doc, res)

	require.Len(t, compLines(doc), 3)
	lineA := compFindLine(t, doc, "emetteurTypeEmetteur", "Société A")
	requireCompDecimal(t, 15, compLineValue(doc, lineA, "montantTotal"))
	requireCompDecimal(t, 2, compLineValue(doc, lineA, "nombreOperations"))
	lineB := compFindLine(t, doc, "emetteurTypeEmetteur", "Société B")
	requireCompDecimal(t, 20, compLineValue(doc, lineB, "montantTotal"))
	lineRef := compFindLine(t, doc, "emetteurTypeEmetteur", "personnes.pp[pp-1]")
	requireCompDecimal(t, 30, compLineValue(doc, lineRef, "montantTotal"))
}

func TestCompute_MoreThan30GroupsBlocks(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	for i := 1; i <= 31; i++ {
		tx := compBase("1", "2026-03-01")
		tx.emetteurLibre = fmt.Sprintf("Émetteur %02d", i) // one group per transaction
		addCompTx(doc, tx)
	}

	res := f.Compute(doc)
	compApply(t, f, doc, res)

	issue, found := findIssue(f.Validate(doc), core.CodeTooManyItems, core.SeverityBlocking, "operations.operation")
	require.True(t, found, "31 operation lines exceed the XSD bound of 30 and must block")
	assert.Contains(t, issue.Message, "regroupez des opérations")
}

func TestCompute_NoTransactionsWritesNothing(t *testing.T) {
	f := compFormat(t)
	// Values computed for an earlier state, with no transaction left.
	doc := core.NewDocument()
	doc.Values["operations.dateDebut"] = core.Value{Value: "2026-01-01", Origin: core.OriginComputed}
	doc.Values["operations.dateFin"] = core.Value{Value: "2026-01-02", Origin: core.OriginComputed}
	doc.Values["operations.montantTotal"] = core.Value{Value: "10", Origin: core.OriginComputed}
	line := doc.AddItem("operations.operation", "operation")
	doc.Values[line.Child("typeOperation")] = core.Value{Value: "OT10", Origin: core.OriginComputed}
	doc.Values[line.Child("montantTotal")] = core.Value{Value: "10", Origin: core.OriginComputed}

	res := f.Compute(doc)
	assert.Empty(t, res.Fills, "without transactions Compute writes no value")
	for _, p := range []core.Path{"operations.dateDebut", "operations.dateFin", "operations.montantTotal", line} {
		covered := false
		for _, r := range res.RemoveItems {
			if p.HasPrefix(r) {
				covered = true
			}
		}
		assert.True(t, covered, "%s is computed and must be removed", p)
	}

	compApply(t, f, doc, res)
	for p := range doc.Values {
		assert.False(t, p.HasPrefix("operations"), "no value may stay under operations: %s", p)
	}
	assert.Empty(t, doc.Items["operations.operation"])

	_, found := findIssue(f.Validate(doc), core.CodeRequired, core.SeverityBlocking, "operations")
	assert.False(t, found, "an operations block without transactions is optional: no period or total is required")
}

func TestCompute_NonEuroExcludedFromSums(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	addCompTx(doc, compBase("100", "2026-03-01"))
	addCompTx(doc, compBase("50", "2026-03-02"))
	usd := compBase("1000", "2026-03-03")
	usd.devise = "USD"
	addCompTx(doc, usd)

	res := f.Compute(doc)
	compApply(t, f, doc, res)

	requireCompDecimal(t, 150, doc.Values["operations.montantTotal"].Value)
	require.Len(t, compLines(doc), 1, "the non-EUR transaction does not create an operation")
	line := compLines(doc)[0]
	requireCompDecimal(t, 150, compLineValue(doc, line, "montantTotal"))
}

func TestCompute_RemovesOperationWhenGroupDisappears(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	addCompTx(doc, compBase("100", "2026-03-01"))
	other := compBase("50", "2026-03-02")
	other.typeOperation = "OT06"
	otherItem := addCompTx(doc, other)

	compApply(t, f, doc, f.Compute(doc))
	require.Len(t, compLines(doc), 2)
	stale := compFindLine(t, doc, "typeOperation", "OT06")

	doc.RemoveItem(otherItem)
	res := f.Compute(doc)
	assert.Contains(t, res.RemoveItems, stale, "a line without a group is listed for removal")
	compApply(t, f, doc, res)

	require.Len(t, compLines(doc), 1)
	for _, l := range compLines(doc) {
		assert.NotEqual(t, "OT06", compLineValue(doc, l, "typeOperation"))
	}
}

func TestCompute_ReusesExistingLineForSameGroup(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	addCompTx(doc, compBase("100", "2026-03-01"))
	other := compBase("50", "2026-03-02")
	other.typeOperation = "OT06"
	addCompTx(doc, other)

	compApply(t, f, doc, f.Compute(doc))
	before := compLines(doc)
	require.Len(t, before, 2)

	// Same document, second computation: same lines, nothing created.
	res := f.Compute(doc)
	for _, fill := range res.Fills {
		assert.False(t, fill.Path.HasProvisionalKey(),
			"an existing line of the same group must be reused, got a new line for %s", fill.Path)
	}
	assert.Empty(t, res.RemoveItems)
	compApply(t, f, doc, res)
	assert.Equal(t, before, compLines(doc), "the same lines keep the same paths")
}

func TestCompute_AutreTypeOperationCopiedToLine(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	tx := compBase("100", "2026-03-01")
	tx.typeOperation = "OT11"
	tx.autreTypeOperation = "Troc de bitcoins"
	addCompTx(doc, tx)

	compApply(t, f, doc, f.Compute(doc))

	line := compFindLine(t, doc, "typeOperation", "OT11")
	assert.Equal(t, "Troc de bitcoins", compLineValue(doc, line, "autretypeOperation"))
	assert.True(t, strings.TrimSpace(compLineValue(doc, line, "autretypeOperation")) != "")
}

// A line whose group gains a transaction keeps its path and its typeOperation
// value, but every attribute of the line must cite the new transactions, not
// only those whose value changed.
func TestCompute_RegroupRefreshesProvenanceOfUnchangedAttributes(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	t1 := addCompTx(doc, compBase("100", "2026-03-01"))
	t2 := addCompTx(doc, compBase("200", "2026-03-02"))

	_, changed := core.ComputeInto(f, doc)
	require.True(t, changed)
	line := compFindLine(t, doc, "typeOperation", "OT10")
	assert.ElementsMatch(t, []core.Path{t1.Child("montant"), t2.Child("montant")},
		doc.Values[line.Child("typeOperation")].Computation.Inputs)

	// Nothing moved: nothing is written, the version must not bump.
	_, changed = core.ComputeInto(f, doc)
	assert.False(t, changed)

	t3 := addCompTx(doc, compBase("50", "2026-03-03"))
	res, changed := core.ComputeInto(f, doc)
	require.True(t, changed)
	assert.NotEmpty(t, res.Refreshed, "typeOperation and the countries keep their value but get new inputs")

	require.Equal(t, []core.Path{line}, compLines(doc), "the line keeps its path")
	wantInputs := []core.Path{t1.Child("montant"), t2.Child("montant"), t3.Child("montant")}
	for _, field := range []string{"typeOperation", "paysOrigineFonds", "paysArriveeFonds", "montantTotal"} {
		v := doc.Values[line.Child(field)]
		require.NotNil(t, v.Computation, field)
		assert.ElementsMatch(t, wantInputs, v.Computation.Inputs, "%s cites the whole new group", field)
	}
	count := doc.Values[line.Child("nombreOperations")]
	require.NotNil(t, count.Computation)
	assert.ElementsMatch(t, []core.Path{t1, t2, t3}, count.Computation.Inputs)
	assert.Equal(t, "OT10", compLineValue(doc, line, "typeOperation"))
	assert.Equal(t, core.OriginComputed, doc.Values[line.Child("typeOperation")].Origin)
}
