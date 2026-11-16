package tracfin_test

import (
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTracfinSpec(t *testing.T) *core.Spec {
	t.Helper()
	spec, ok := tracfin.Spec(tracfin.CurrentVersion)
	require.True(t, ok)
	return spec
}

func setTracfinValue(d *core.Document, path core.Path, raw string) {
	d.Values[path] = core.Value{Value: raw, Origin: core.OriginManual}
}

// findIssue reports whether an issue with this code, severity and path prefix exists.
func findIssue(issues []core.Issue, code string, severity core.Severity, prefix core.Path) (core.Issue, bool) {
	for _, is := range issues {
		if is.Code == code && is.Severity == severity && is.Path.HasPrefix(prefix) {
			return is, true
		}
	}
	return core.Issue{}, false
}

func TestValidate_EmptyDocumentListsMandatoryFields(t *testing.T) {
	spec := loadTracfinSpec(t)
	issues := tracfin.Validate(spec, core.NewDocument())

	for _, prefix := range []core.Path{"organisme", "context", "analyse"} {
		_, found := findIssue(issues, "required", core.SeverityBlocking, prefix)
		assert.True(t, found, "expected a blocking required issue under %s", prefix)
	}
}

func TestValidate_UnknownCode(t *testing.T) {
	spec := loadTracfinSpec(t)
	d := core.NewDocument()
	setTracfinValue(d, "organisme.profession", "ZZZ999")

	is, found := findIssue(tracfin.Validate(spec, d), "unknown_code", core.SeverityBlocking, "organisme.profession")
	require.True(t, found, "ZZZ999 is not in liste_ref_professions.csv")
	assert.Equal(t, core.Path("organisme.profession"), is.Path)
}

func TestValidate_AnalyseTooLong(t *testing.T) {
	spec := loadTracfinSpec(t)
	d := core.NewDocument()
	setTracfinValue(d, "analyse", strings.Repeat("a", 23001))

	_, found := findIssue(tracfin.Validate(spec, d), "max_length", core.SeverityBlocking, "analyse")
	assert.True(t, found, "analyse is limited to 23000 characters")
}

func TestValidate_TooManyOperations(t *testing.T) {
	spec := loadTracfinSpec(t)
	d := core.NewDocument()
	for i := 0; i < 31; i++ {
		d.AddItem("operations.operation", "operation")
	}

	_, found := findIssue(tracfin.Validate(spec, d), "too_many_items", core.SeverityBlocking, "operations.operation")
	assert.True(t, found, "operations.operation allows at most 30 items")
}

func TestValidate_DanglingRef(t *testing.T) {
	spec := loadTracfinSpec(t)
	d := core.NewDocument()
	tx := d.AddItem("work.transactions", "transactions")
	setTracfinValue(d, tx.Child("emetteurPersonne"), "personnes.pp[pp-9]")

	_, found := findIssue(tracfin.Validate(spec, d), "dangling_ref", core.SeverityBlocking, tx)
	assert.True(t, found, "emetteurPersonne targets an item that does not exist")

	// Same reference, now to an existing person: no dangling_ref.
	ok := core.NewDocument()
	pp := ok.AddItem("personnes.pp", "pp")
	tx2 := ok.AddItem("work.transactions", "transactions")
	setTracfinValue(ok, tx2.Child("emetteurPersonne"), string(pp))
	for _, is := range tracfin.Validate(spec, ok) {
		assert.NotEqual(t, "dangling_ref", is.Code, "unexpected dangling_ref on %s", is.Path)
	}
}

func TestValidate_ConditionalNSPRequiresCL02(t *testing.T) {
	spec := loadTracfinSpec(t)

	// CL02 declared, no nature of penal suspicion: required by the XSD rule.
	withCL02 := core.NewDocument()
	cl := withCL02.AddItem("context.natureSoupcon.cadreLegal", "cl")
	setTracfinValue(withCL02, cl, "CL02")
	var found bool
	for _, is := range tracfin.Validate(spec, withCL02) {
		if is.Code == "rule:nsp_requise_si_cl02" {
			found = true
			assert.Equal(t, core.SeverityBlocking, is.Severity)
		}
	}
	assert.True(t, found, "NSP is required when CL02 is declared")

	// CL01 only, but a penal nature of suspicion: forbidden without CL02.
	withoutCL02 := core.NewDocument()
	cl1 := withoutCL02.AddItem("context.natureSoupcon.cadreLegal", "cl")
	setTracfinValue(withoutCL02, cl1, "CL01")
	nsp := withoutCL02.AddItem("context.natureSoupcon.natureSoupconPenale", "nsp")
	setTracfinValue(withoutCL02, nsp, "NSP01")
	found = false
	for _, is := range tracfin.Validate(spec, withoutCL02) {
		if is.Code == "rule:nsp_interdite_hors_cl02" {
			found = true
			assert.Equal(t, core.SeverityBlocking, is.Severity)
		}
	}
	assert.True(t, found, "NSP is only allowed when CL02 is declared")
}

func TestValidate_HandRuleCadreLegalPrincipal(t *testing.T) {
	spec := loadTracfinSpec(t)
	d := core.NewDocument()
	cl := d.AddItem("context.natureSoupcon.cadreLegal", "cl")
	setTracfinValue(d, cl, "CL04")

	var found bool
	for _, is := range tracfin.Validate(spec, d) {
		if is.Code == "rule:cadre_legal_principal" {
			found = true
			assert.Equal(t, core.SeverityBlocking, is.Severity)
		}
	}
	assert.True(t, found, "CL04 alone does not satisfy the main legal ground")
}

func TestValidate_NonEuroAmountBlocks(t *testing.T) {
	spec := loadTracfinSpec(t)

	usd := core.NewDocument()
	tx := usd.AddItem("work.transactions", "transactions")
	setTracfinValue(usd, tx.Child("devise"), "USD")
	setTracfinValue(usd, tx.Child("montant"), "10")

	is, found := findIssue(tracfin.Validate(spec, usd), "montant_hors_euro", core.SeverityBlocking, tx)
	require.True(t, found, "a non-EUR transaction must block on its montant")
	assert.Equal(t, tx.Child("montant"), is.Path)

	eur := core.NewDocument()
	tx2 := eur.AddItem("work.transactions", "transactions")
	setTracfinValue(eur, tx2.Child("devise"), "EUR")
	setTracfinValue(eur, tx2.Child("montant"), "10")
	_, found = findIssue(tracfin.Validate(spec, eur), "montant_hors_euro", core.SeverityBlocking, tx2)
	assert.False(t, found, "EUR transactions are not concerned")
}

func TestValidate_TransactionOT06OT07(t *testing.T) {
	spec := loadTracfinSpec(t)
	has := func(issues []core.Issue, id string, tx core.Path) bool {
		for _, is := range issues {
			if is.Code == "rule:"+id && is.Path.HasPrefix(tx) {
				return true
			}
		}
		return false
	}

	// OT07 (cash deposit): no issuer required, a beneficiary still is.
	ot07 := core.NewDocument()
	tx := ot07.AddItem("work.transactions", "transactions")
	setTracfinValue(ot07, tx.Child("typeOperation"), "OT07")
	issues := tracfin.Validate(spec, ot07)
	assert.False(t, has(issues, "transaction_emetteur_requis_sauf_ot07", tx))
	assert.True(t, has(issues, "transaction_beneficiaire_requis_sauf_ot06", tx))

	// OT06: no beneficiary required, an issuer is.
	ot06 := core.NewDocument()
	tx6 := ot06.AddItem("work.transactions", "transactions")
	setTracfinValue(ot06, tx6.Child("typeOperation"), "OT06")
	issues = tracfin.Validate(spec, ot06)
	assert.False(t, has(issues, "transaction_beneficiaire_requis_sauf_ot06", tx6))
	assert.True(t, has(issues, "transaction_emetteur_requis_sauf_ot07", tx6))

	// Both issuer forms set: the exclusive at_most_one rule applies regardless of the type.
	both := core.NewDocument()
	pp := both.AddItem("personnes.pp", "pp")
	tx2 := both.AddItem("work.transactions", "transactions")
	setTracfinValue(both, tx2.Child("typeOperation"), "OT07")
	setTracfinValue(both, tx2.Child("emetteurPersonne"), string(pp))
	setTracfinValue(both, tx2.Child("emetteurLibre"), "texte")
	assert.True(t, has(tracfin.Validate(spec, both), "transaction_emetteur_exclusif", tx2))
}

func TestValidate_HandRulesPersonnesAndReferences(t *testing.T) {
	spec := loadTracfinSpec(t)
	d := core.NewDocument()
	d.NextKey["personnes.pp"] = 0
	pp := d.AddItem("personnes.pp", "pp")
	for i := 0; i < 21; i++ {
		d.AddItem(pp.Child("referencesFinancieres.referenceFinanciereIban"), "ref")
	}
	_, found := findIssue(tracfin.Validate(spec, d), "rule:references_financieres_max_20", core.SeverityBlocking, pp)
	assert.True(t, found)
}

func TestValidate_HandRuleSousCategorieAutre(t *testing.T) {
	spec := loadTracfinSpec(t)
	d := core.NewDocument()
	pp := d.AddItem("personnes.pp", "pp")
	lien := d.AddItem(pp.Child("lienAutrePp"), "lien")
	rel := d.AddItem(lien.Child("typeRelation"), "rel")
	setTracfinValue(d, rel.Child("categorieRelation"), "TLP54")
	setTracfinValue(d, rel.Child("sousCategorieRelation"), "TLP1")
	setTracfinValue(d, rel.Child("sousCategorieRelationAutre"), "précision")
	_, found := findIssue(tracfin.Validate(spec, d), "rule:sous_categorie_relation_autre", core.SeverityWarning, rel)
	assert.True(t, found, "precision without an 'autre' sub-category")

	setTracfinValue(d, rel.Child("sousCategorieRelation"), "TLP11")
	_, found = findIssue(tracfin.Validate(spec, d), "rule:sous_categorie_relation_autre", core.SeverityWarning, rel)
	assert.False(t, found)
}
