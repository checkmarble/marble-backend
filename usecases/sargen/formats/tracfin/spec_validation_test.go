// Validation of real values against the TRACFIN spec: code lists, facets,
// references, rules and the unknown country.
package tracfin_test

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// concretePath turns a template into a path (`[]` becomes `[k-1]`).
func concretePath(template string) core.Path {
	return core.Path(strings.ReplaceAll(template, "[]", "[k-1]"))
}

func TestPaysAutreCodeIsExcluded(t *testing.T) {
	spec := currentSpec(t)
	for _, name := range []string{"liste_ref_pays.csv", "liste_ref_dromcom.csv+liste_ref_pays.csv"} {
		for _, c := range spec.CodeLists[name].Codes {
			assert.NotEqual(t, "1", c.Value, "%s : le code 1 (Autre) n'a pas la longueur de t_pays", name)
		}
	}
	// A country node carries the XSD length facets and refuses the code.
	node := spec.NodeAt("organisme.adresse.pays")
	require.NotNil(t, node)
	assert.Equal(t, 2, node.MinLength)
	assert.Equal(t, 200, node.MaxLength)
	issue := core.ValidateValue(spec, "organisme.adresse.pays", "1")
	require.NotNil(t, issue)
	assert.True(t, issue.Blocks())
	assert.Nil(t, core.ValidateValue(spec, "organisme.adresse.pays", "FR"))
}

// Every code of every code list satisfies the facets of every node that uses
// it: the validator never blocks a published code (the generator fails first).
func TestEveryPublishedCodeIsAccepted(t *testing.T) {
	spec := currentSpec(t)
	walkNodes(spec.Root, func(n *core.Node) {
		if n.Type != core.TypeCode || !n.Export || n.ExportID {
			return
		}
		for _, c := range spec.CodeLists[n.CodeList].Codes {
			issue := core.ValidateValue(spec, concretePath(n.Template), c.Value)
			assert.False(t, issue != nil && issue.Blocks(), "%s refuse le code %q de %s : %v", n.Template, c.Value, n.CodeList, issue)
		}
	})
}

func TestPositiveIntegersRefuseZero(t *testing.T) {
	spec := currentSpec(t)
	count := 0
	walkNodes(spec.Root, func(n *core.Node) {
		if n.Type != core.TypeInteger || !n.Export || n.ExportID {
			return
		}
		count++
		p := concretePath(n.Template)
		for _, raw := range []string{"0", "000"} {
			issue := core.ValidateValue(spec, p, raw)
			require.NotNil(t, issue, "%s accepte %q alors que le XSD (xs:positiveInteger) le refuse", n.Template, raw)
			assert.True(t, issue.Blocks(), n.Template)
		}
		assert.Nil(t, core.ValidateValue(spec, p, "7"), n.Template)
	})
	assert.GreaterOrEqual(t, count, 10, "adresse.numero, numeroTeleDS, aav.reference, nombreOperations…")
	assert.Nil(t, core.ValidateValue(spec, "organisme.adresse.numero", "000012"), "zéros de tête : 2 chiffres significatifs")
}

func TestIllegalXMLCharsBlock(t *testing.T) {
	spec := currentSpec(t)
	d := core.NewDocument()
	setTracfinValue(d, "organisme.libelle", "\x01")

	is, found := findIssue(tracfin.Validate(spec, d), "bad_char", core.SeverityBlocking, "organisme.libelle")
	require.True(t, found, "une valeur faite de caractères interdits en XML ne doit pas être écrite vide")
	assert.Equal(t, core.Path("organisme.libelle"), is.Path)
}

func TestWrongKindRefIsBadRef(t *testing.T) {
	spec := currentSpec(t)
	d := core.NewDocument()
	pp := d.AddItem("personnes.pp", "pp")
	op := d.AddItem("operations.operation", "op")

	// A person is not a financial reference: neither an id nor free text.
	setTracfinValue(d, op.Child("refFinanciereEmission"), string(pp))
	_, found := findIssue(tracfin.Validate(spec, d), "bad_ref", core.SeverityBlocking, op.Child("refFinanciereEmission"))
	assert.True(t, found, "chemin d'une personne dans un champ de référence financière")

	// Same for a path to an item that does not exist.
	setTracfinValue(d, op.Child("refFinanciereEmission"), "personnes.pp[pp-9]")
	_, found = findIssue(tracfin.Validate(spec, d), "bad_ref", core.SeverityBlocking, op.Child("refFinanciereEmission"))
	assert.True(t, found)

	// Free text stays free text.
	setTracfinValue(d, op.Child("refFinanciereEmission"), "virement du 12 mars")
	_, found = findIssue(tracfin.Validate(spec, d), "bad_ref", core.SeverityBlocking, op.Child("refFinanciereEmission"))
	assert.False(t, found)

	// Plain ref node: a firm is not a financial reference either.
	tx := d.AddItem("work.transactions", "tx")
	setTracfinValue(d, tx.Child("emetteurPersonne"), "operations.operation[op-1]")
	_, found = findIssue(tracfin.Validate(spec, d), "bad_ref", core.SeverityBlocking, tx.Child("emetteurPersonne"))
	assert.True(t, found)
}

func TestOT11TransactionRequiresAutreTypeOperation(t *testing.T) {
	spec := currentSpec(t)
	node := spec.NodeAt("work.transactions[].autretypeOperation")
	require.NotNil(t, node, "champ de travail source de la précision des lignes d'opération")
	assert.Equal(t, 200, node.MaxLength)
	assert.False(t, node.Export)

	d := core.NewDocument()
	tx := d.AddItem("work.transactions", "tx")
	setTracfinValue(d, tx.Child("typeOperation"), "OT11")

	const rule = "rule:transaction_autre_type_requis_ot11"
	is, found := findIssue(tracfin.Validate(spec, d), rule, core.SeverityWarning, tx)
	require.True(t, found)
	assert.Equal(t, tx.Child("autretypeOperation"), is.Path)

	setTracfinValue(d, tx.Child("autretypeOperation"), "Remise de bijoux")
	_, found = findIssue(tracfin.Validate(spec, d), rule, core.SeverityWarning, tx)
	assert.False(t, found)

	setTracfinValue(d, tx.Child("typeOperation"), "OT01")
	setTracfinValue(d, tx.Child("autretypeOperation"), "")
	_, found = findIssue(tracfin.Validate(spec, d), rule, core.SeverityWarning, tx)
	assert.False(t, found, "pas d'obligation pour un autre type d'opération")
}

func TestHandRulesAreImplemented(t *testing.T) {
	raw, err := os.ReadFile("versions/" + tracfin.CurrentVersion + "/constraints.json")
	require.NoError(t, err)
	var c struct {
		HandRules []struct {
			ID string `json:"id"`
		} `json:"hand_rules"`
	}
	require.NoError(t, json.Unmarshal(raw, &c))
	var declared []string
	for _, r := range c.HandRules {
		declared = append(declared, r.ID)
	}
	implemented := tracfin.HandRuleIDs()
	slices.Sort(declared)
	slices.Sort(implemented)
	assert.Equal(t, declared, implemented, "chaque hand_rules de constraints.json a son implémentation Go, et inversement")
}

func TestRefTargetsCarryAnExportID(t *testing.T) {
	spec := currentSpec(t)
	checked := 0
	walkNodes(spec.Root, func(n *core.Node) {
		for _, target := range n.RefTargets {
			node := spec.NodeAt(target)
			require.NotNil(t, node, "%s -> %s", n.Template, target)
			hasID := slices.ContainsFunc(node.Children, func(c *core.Node) bool { return c.ExportID })
			assert.True(t, hasID, "%s : la cible %s n'a pas d'identifiant d'export", n.Template, target)
			assert.True(t, node.IsRepeatable(), target)
			checked++
		}
	})
	assert.Positive(t, checked)
}

// "Inconnu" (XX) is a published line of the country list: the validator accepts
// it on the country nodes, and the country lists declare it as their unknown code.
func TestUnknownCountryIsAcceptedAndDeclared(t *testing.T) {
	spec := currentSpec(t)
	for _, name := range []string{"liste_ref_pays.csv", "liste_ref_dromcom.csv+liste_ref_pays.csv"} {
		list := spec.CodeLists[name]
		assert.Equal(t, "XX", list.UnknownCode, name)
		assert.True(t, slices.ContainsFunc(list.Codes, func(c core.Code) bool { return c.Value == "XX" && c.Label == "Inconnu" }), name)
	}
	assert.Nil(t, core.ValidateValue(spec, "work.transactions[k-1].paysOrigine", "XX"))
	assert.Nil(t, core.ValidateValue(spec, "organisme.adresse.pays", "XX"))
}

// « Je ne sais pas » as register type may come without a number: the address is
// then not required. With a number, it is.
func TestRegisterAddressRequiredOnlyWithANumber(t *testing.T) {
	f, ok := tracfin.New(tracfin.CurrentVersion)
	require.True(t, ok)
	d := core.NewDocument()
	pm := d.AddItem("personnes.pm", "pm")
	scope := pm.Child("activiteCoordonnee")
	setTracfinValue(d, scope.Child("numeroIdentification").Child("type"), "Je ne sais pas")
	hasRule := func() bool {
		for _, is := range f.Validate(d) {
			if strings.HasPrefix(is.Code, "rule:registre_adresse_requise_pm") && is.Path.HasPrefix(scope) {
				return true
			}
		}
		return false
	}
	assert.False(t, hasRule(), "no number: no address required")
	setTracfinValue(d, scope.Child("numeroIdentification").Child("_text"), "123456789")
	assert.True(t, hasRule(), "a number: the address is required")
}

// The professional address of a natural person goes with a professional
// number: the register type alone (« Je ne sais pas » included) is not enough.
func TestNaturalPersonAddressWarnsWithoutANumber(t *testing.T) {
	f, ok := tracfin.New(tracfin.CurrentVersion)
	require.True(t, ok)
	d := core.NewDocument()
	pp := d.AddItem("personnes.pp", "pp")
	act := pp.Child("activite")
	setTracfinValue(d, act.Child("numeroIdentification").Child("type"), "Je ne sais pas")
	setTracfinValue(d, act.Child("adresse").Child("ville"), "Lyon")
	warned := func() bool {
		_, found := findIssue(f.Validate(d), "rule:registre_adresse_pp_si_numero", core.SeverityWarning, act)
		return found
	}
	assert.True(t, warned(), "a register type without a number: the address is flagged")
	setTracfinValue(d, act.Child("numeroIdentification").Child("_text"), "123456789")
	assert.False(t, warned(), "a number: the address is expected")
}
