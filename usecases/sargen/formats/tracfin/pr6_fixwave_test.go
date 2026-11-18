package tracfin_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/engine"
)

// blockingUnder returns the blocking issues whose path is prefix or under it.
func blockingUnder(issues []core.Issue, prefix core.Path) []core.Issue {
	var out []core.Issue
	for _, is := range issues {
		if is.Blocks() && is.Path.HasPrefix(prefix) {
			out = append(out, is)
		}
	}
	return out
}

// T1 (PR6 proof item 4): a working transaction that Compute does not count
// (missing currency, missing amount) is a blocking issue on that transaction.

func TestValidate_TransactionWithoutCurrencyBlocks(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	b := compBase("100", "2026-03-01")
	b.devise = ""
	b.emetteurLibre, b.beneficiaireLibre = "A", "B"
	tx := addCompTx(doc, b)

	assert.NotEmpty(t, blockingUnder(f.Validate(doc), tx),
		"a transaction without currency is not counted: it must block the export")
}

func TestValidate_TransactionWithoutAmountBlocks(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	b := compBase("", "2026-03-01")
	b.emetteurLibre, b.beneficiaireLibre = "A", "B"
	tx := addCompTx(doc, b)

	assert.NotEmpty(t, blockingUnder(f.Validate(doc), tx),
		"a transaction without amount is not counted: it must block the export")
}

// Guard: the deliberate foreign-currency exclusion keeps its single blocking
// issue (montant_hors_euro) and gains no second one from the uncounted rule.
func TestValidate_ForeignCurrencyTransactionBlocksOnlyOnce(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	b := compBase("100", "2026-03-01")
	b.devise = "USD"
	b.emetteurLibre, b.beneficiaireLibre = "A", "B"
	tx := addCompTx(doc, b)

	blocking := blockingUnder(f.Validate(doc), tx)
	require.Len(t, blocking, 1, "only the foreign-currency rule blocks")
	assert.Equal(t, "montant_hors_euro", blocking[0].Code)
}

// All transactions without currency: nothing is computed, and the export must
// be refused by a blocking issue on the transactions or the operations.
func TestValidate_NoCountableTransactionBlocksExport(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	for _, amount := range []string{"100", "200"} {
		b := compBase(amount, "2026-03-01")
		b.devise = ""
		b.emetteurLibre, b.beneficiaireLibre = "A", "B"
		addCompTx(doc, b)
	}
	_, _ = core.ComputeInto(f, doc)
	require.Empty(t, doc.Items["operations.operation"], "no transaction is counted")

	var workOrOps []core.Issue
	for _, is := range f.Validate(doc) {
		if is.Blocks() && (is.Path.HasPrefix("work") || is.Path.HasPrefix("operations")) {
			workOrOps = append(workOrOps, is)
		}
	}
	assert.NotEmpty(t, workOrOps, "transactions were selected but none is counted: the export must be refused")
}

// T2 (PR6 proof item 5, 4.1) is an end-to-end test: the placement output no
// longer lists the account, and the backend must add it from the data model.
// ApplyStructured has no selection or data model today; see
// usecases/sargen/pr6_fixwave_test.go.

// T3 (PR6 proof item 6): the placement dedupes on the transaction object and
// merges identical persons and references only.

func TestApplyPlacement_SameTransactionListedTwiceIsOneTransaction(t *testing.T) {
	f := planFormat(t)
	doc := core.NewDocument()
	raw := json.RawMessage(`{"persons":[],"financial_refs":[],"transactions":[
	  {"objects":[{"table":"transactions","object_id":"t-1"},{"table":"accounts","object_id":"a-1"}]},
	  {"objects":[{"table":"transactions","object_id":"t-1"},{"table":"accounts","object_id":"a-2"}]}]}`)
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, core.StructuredContext{}))

	require.Len(t, doc.Items["work.transactions"], 1, "one transaction object is one working transaction")
	tx := core.Path("work.transactions[" + doc.Items["work.transactions"][0] + "]")
	assert.Contains(t, doc.ItemSources[tx], core.ObjectRef{Table: "transactions", ObjectID: "t-1"})
}

func TestApplyPlacement_IdenticalPersonEntriesAreOnePerson(t *testing.T) {
	f := planFormat(t)
	doc := core.NewDocument()
	raw := json.RawMessage(`{"persons":[
	  {"kind":"pp","objects":[{"table":"clients","object_id":"c-1"}]},
	  {"kind":"pp","objects":[{"table":"clients","object_id":"c-1"}]}],
	 "financial_refs":[],"transactions":[]}`)
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, core.StructuredContext{}))

	assert.Len(t, doc.Items["personnes.pp"], 1, "two identical entries describe one person")
}

// Guard: an object may describe two different persons (a company and its
// manager, or two persons sharing a client object).
func TestApplyPlacement_ObjectSharedByDistinctPersonsStaysAllowed(t *testing.T) {
	f := planFormat(t)
	doc := core.NewDocument()
	raw := json.RawMessage(`{"persons":[
	  {"kind":"pm","objects":[{"table":"companies","object_id":"co-1"}]},
	  {"kind":"pp","objects":[{"table":"companies","object_id":"co-1"}]},
	  {"kind":"pp","objects":[{"table":"clients","object_id":"c-1"}]},
	  {"kind":"pp","objects":[{"table":"clients","object_id":"c-1"},{"table":"clients","object_id":"c-2"}]}],
	 "financial_refs":[],"transactions":[]}`)
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, core.StructuredContext{}))

	assert.Len(t, doc.Items["personnes.pm"], 1)
	assert.Len(t, doc.Items["personnes.pp"], 3, "a shared object does not merge distinct persons")
}

func TestApplyPlacement_IdenticalFinancialReferencesAreOne(t *testing.T) {
	f := planFormat(t)
	doc := core.NewDocument()
	ref := `{"ref_kind":"Iban","owners":[{"person_index":0,"lien":"titulaire"}],"objects":[{"table":"accounts","object_id":"a-1"}]}`
	raw := json.RawMessage(`{"persons":[{"kind":"pp","objects":[{"table":"clients","object_id":"c-1"}]}],
	 "financial_refs":[` + ref + `,` + ref + `],"transactions":[]}`)
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, core.StructuredContext{}))

	assert.Len(t, doc.Items["personnes.pp[pp-1].referencesFinancieres.referenceFinanciereIban"], 1,
		"two identical references under the same owner are one")
}

// Guard: the same account declared under two owners is one reference per owner.
func TestApplyPlacement_SameObjectsUnderTwoOwnersStayTwoReferences(t *testing.T) {
	f := planFormat(t)
	doc := core.NewDocument()
	raw := json.RawMessage(`{"persons":[
	  {"kind":"pp","objects":[{"table":"clients","object_id":"c-1"}]},
	  {"kind":"pp","objects":[{"table":"clients","object_id":"c-2"}]}],
	 "financial_refs":[
	  {"ref_kind":"Iban","owners":[{"person_index":0,"lien":"titulaire"}],"objects":[{"table":"accounts","object_id":"a-1"}]},
	  {"ref_kind":"Iban","owners":[{"person_index":1,"lien":"co-titulaire"}],"objects":[{"table":"accounts","object_id":"a-1"}]}],
	 "transactions":[]}`)
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, core.StructuredContext{}))

	assert.Len(t, doc.Items["personnes.pp[pp-1].referencesFinancieres.referenceFinanciereIban"], 1)
	assert.Len(t, doc.Items["personnes.pp[pp-2].referencesFinancieres.referenceFinanciereIban"], 1)
}

// T4 (PR6 proof item 7): fill_person carries the declared persons and
// references block, and every tool of its list exists.

func pr6PersonDoc() (doc *core.Document, pm, pp core.Path) {
	doc = core.NewDocument()
	pm = doc.AddItem("personnes.pm", "pm")
	doc.ItemSources[pm] = []core.ObjectRef{{Table: "companies", ObjectID: "co-1"}}
	pp = doc.AddItem("personnes.pp", "pp")
	doc.ItemSources[pp] = []core.ObjectRef{{Table: "clients", ObjectID: "c-1"}}
	return doc, pm, pp
}

func pr6FillPersonSpecs(t *testing.T, f core.Format, doc *core.Document) []core.TaskSpec {
	t.Helper()
	specs := f.Plan(core.PlanState{JobKind: string(models.SarJobKindGenerate), Doc: doc, Done: planPlacementDone()})
	out := planWaveOf(specs, stepFillPerson)
	require.Len(t, out, 2, "one fill_person per person")
	return out
}

func TestPlan_FillPersonCarriesDeclaredPersonsBlock(t *testing.T) {
	f := planFormat(t)
	doc, pm, pp := pr6PersonDoc()
	for _, s := range pr6FillPersonSpecs(t, f, doc) {
		block, ok := s.Payload["declared_persons_and_references"]
		require.True(t, ok, "fill_person of %s has the declared_persons_and_references block", s.TargetPath)
		raw, err := json.Marshal(block)
		require.NoError(t, err)
		assert.Contains(t, string(raw), string(pm), "the block lists the declared company")
		assert.Contains(t, string(raw), string(pp), "the block lists the declared person")
	}
}

func TestPlan_FillPersonToolsExist(t *testing.T) {
	f := planFormat(t)
	doc, _, _ := pr6PersonDoc()
	tc := core.ToolContext{Format: f, Doc: doc}
	available := map[string]bool{}
	for name := range engine.CommonTools(tc) {
		available[name] = true
	}
	for name := range f.Tools(tc) {
		available[name] = true
	}
	for _, s := range pr6FillPersonSpecs(t, f, doc) {
		require.NotEmpty(t, s.ToolNames)
		for _, name := range s.ToolNames {
			assert.True(t, available[name], "fill_person of %s lists tool %q, which does not exist", s.TargetPath, name)
		}
	}
}

// T5 (PR6 jobs item 5): the placement ends the job when it stops on a limit.
// The budget growth with the selection size is covered end to end in
// usecases/sargen/pr6_fixwave_test.go.

func TestPlan_PlacementFailsTheJobOnLimit(t *testing.T) {
	f := planFormat(t)
	specs := f.Plan(core.PlanState{JobKind: string(models.SarJobKindGenerate), Doc: core.NewDocument()})
	require.Len(t, specs, 1)
	require.Equal(t, stepPlacement, specs[0].Kind)
	assert.True(t, specs[0].FailJobOnLimit, "a placement ending on a limit must fail the job")
}
