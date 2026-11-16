package core_test

import (
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Same shape as the TRACFIN phone pattern once its XSD anchors are stripped.
	phonePattern = `[+]*[(]{0,1}[0-9]{1,4}[)]{0,1}[-\s\./0-9]*`
	emailPattern = `[\w-\._]+@([\w-]+\.)+[\w-]{2,4}`
)

// validationSpec is a small hand-made spec covering the generic validator
// features: groups, a repeatable with Max, date, pattern, code lists (bound and
// unavailable), a TypeRef, a blocking required_if rule and a warning rule.
func validationSpec() *core.Spec {
	spec := rawValidationSpec()
	markExported(spec.Root)
	return spec
}

// markExported sets Export on every node: the tree below describes exported
// nodes (Export false would make their obligations warnings).
func markExported(n *core.Node) {
	n.Export = true
	for _, c := range n.Children {
		markExported(c)
	}
}

func rawValidationSpec() *core.Spec {
	return &core.Spec{
		Key: core.FormatKey{Family: "test", Country: "FR", Version: "0"},
		CodeLists: map[string]core.CodeList{
			"liste_test_profession.csv": {
				Name:  "liste_test_profession.csv",
				Codes: []core.Code{{Value: "ACT", Label: "Actif"}, {Value: "INA", Label: "Inactif"}},
			},
			"liste_test_equipement.csv": {Name: "liste_test_equipement.csv", Unavailable: true},
		},
		Root: &core.Node{Children: []*core.Node{
			{Name: "organisme", Template: "organisme", Kind: core.NodeGroup, Min: 1, Max: 1, Children: []*core.Node{
				{Name: "profession", Template: "organisme.profession", Kind: core.NodeField, Type: core.TypeCode,
					CodeList: "liste_test_profession.csv", Min: 1, Max: 1},
				{Name: "libelle", Template: "organisme.libelle", Kind: core.NodeField, Type: core.TypeString,
					MinLength: 1, MaxLength: 30, Min: 0, Max: 1},
				{Name: "dateCreation", Template: "organisme.dateCreation", Kind: core.NodeField, Type: core.TypeDate,
					Min: 0, Max: 1},
				{Name: "tel", Template: "organisme.tel", Kind: core.NodeField, Type: core.TypeString,
					Pattern: phonePattern, MaxLength: 25, Min: 0, Max: 1},
				{Name: "courriel", Template: "organisme.courriel", Kind: core.NodeField, Type: core.TypeString,
					Pattern: emailPattern, MaxLength: 200, Min: 0, Max: 1},
				{Name: "equipement", Template: "organisme.equipement", Kind: core.NodeField, Type: core.TypeCode,
					CodeList: "liste_test_equipement.csv", Min: 0, Max: 1},
			}},
			{Name: "personnes", Template: "personnes", Kind: core.NodeGroup, Min: 0, Max: 1, Children: []*core.Node{
				{Name: "pp", Template: "personnes.pp[]", Kind: core.NodeGroup, Min: 0, Max: 3, Children: []*core.Node{
					{Name: "nom", Template: "personnes.pp[].nom", Kind: core.NodeField, Type: core.TypeString,
						MaxLength: 50, Min: 1, Max: 1},
				}},
			}},
			{Name: "emetteur", Template: "emetteur", Kind: core.NodeField, Type: core.TypeRef,
				RefTargets: []string{"personnes.pp[]"}, Min: 0, Max: 1},
		}},
		Rules: []core.Rule{
			{
				ID:   "libelle_requis_si_act",
				Kind: "required_if",
				When: &core.Condition{Path: "organisme.profession", Op: "eq", Values: []string{"ACT"}},
				Then: []string{"organisme.libelle"},
			},
			{
				ID:       "courriel_recommande_si_act",
				Kind:     "required_if",
				Severity: core.SeverityWarning,
				When:     &core.Condition{Path: "organisme.profession", Op: "eq", Values: []string{"ACT"}},
				Then:     []string{"organisme.courriel"},
			},
		},
	}
}

func setValue(d *core.Document, path core.Path, raw string) {
	d.Values[path] = core.Value{Value: raw, Origin: core.OriginManual}
}

func blockingIssues(issues []core.Issue) []core.Issue {
	var out []core.Issue
	for _, is := range issues {
		if is.Severity == core.SeverityBlocking {
			out = append(out, is)
		}
	}
	return out
}

func TestValidateValue_Date(t *testing.T) {
	spec := validationSpec()
	for _, raw := range []string{"2026-13-01", "2026-02-31"} {
		issue := core.ValidateValue(spec, "organisme.dateCreation", raw)
		require.NotNil(t, issue, raw)
		assert.Equal(t, "bad_date", issue.Code, raw)
		assert.Equal(t, core.SeverityBlocking, issue.Severity, raw)
	}
	assert.Nil(t, core.ValidateValue(spec, "organisme.dateCreation", "2026-11-01"))
}

func TestValidateValue_LengthCountsCharacters(t *testing.T) {
	spec := validationSpec()
	// 30 characters, 60 bytes: the limit is in characters, not bytes.
	accented := strings.Repeat("é", 30)
	assert.Nil(t, core.ValidateValue(spec, "organisme.libelle", accented))

	issue := core.ValidateValue(spec, "organisme.libelle", strings.Repeat("é", 31))
	require.NotNil(t, issue)
	assert.Equal(t, "max_length", issue.Code)
}

func TestValidateValue_PatternIsAnchored(t *testing.T) {
	spec := validationSpec()

	issue := core.ValidateValue(spec, "organisme.tel", "xx+33 1 23 45 67 89yy")
	require.NotNil(t, issue, "a pattern match inside the value must not be accepted")
	assert.Equal(t, "pattern", issue.Code)

	assert.Nil(t, core.ValidateValue(spec, "organisme.tel", "+33 1 23 45 67 89"))
	assert.Nil(t, core.ValidateValue(spec, "organisme.courriel", "jean.dupont@banque.fr"))
}

func TestValidate_WarningRuleDoesNotBlock(t *testing.T) {
	spec := validationSpec()
	d := core.NewDocument()
	setValue(d, "organisme.profession", "ACT")
	setValue(d, "organisme.libelle", "Cabinet")
	// courriel is missing: the warning rule fires, nothing blocks.

	issues := core.Validate(spec, d)
	assert.Empty(t, blockingIssues(issues))

	var warnings []core.Issue
	for _, is := range issues {
		if is.Code == "rule:courriel_recommande_si_act" {
			warnings = append(warnings, is)
		}
	}
	require.Len(t, warnings, 1)
	assert.Equal(t, core.SeverityWarning, warnings[0].Severity)
}

func TestValidate_RequiredIfRuleBlocks(t *testing.T) {
	spec := validationSpec()

	d := core.NewDocument()
	setValue(d, "organisme.profession", "ACT")
	var found bool
	for _, is := range core.Validate(spec, d) {
		if is.Code == "rule:libelle_requis_si_act" {
			found = true
			assert.Equal(t, core.SeverityBlocking, is.Severity)
		}
	}
	assert.True(t, found, "libelle is required when profession is ACT")

	inactive := core.NewDocument()
	setValue(inactive, "organisme.profession", "INA")
	for _, is := range core.Validate(spec, inactive) {
		assert.NotEqual(t, "rule:libelle_requis_si_act", is.Code)
	}
}

func TestValidate_DanglingRef(t *testing.T) {
	spec := validationSpec()

	dangling := core.NewDocument()
	setValue(dangling, "emetteur", "personnes.pp[pp-9]")
	var found bool
	for _, is := range core.Validate(spec, dangling) {
		if is.Code == "dangling_ref" {
			found = true
		}
	}
	assert.True(t, found, "reference to an absent item must be reported")

	ok := core.NewDocument()
	key := ok.AddItem("personnes.pp", "pp")
	setValue(ok, "emetteur", string(key))
	for _, is := range core.Validate(spec, ok) {
		assert.NotEqual(t, "dangling_ref", is.Code)
	}
}

func TestValidate_TooManyItems(t *testing.T) {
	spec := validationSpec()
	d := core.NewDocument()
	for i := 0; i < 4; i++ {
		d.AddItem("personnes.pp", "pp")
	}
	var found bool
	for _, is := range core.Validate(spec, d) {
		if is.Code == "too_many_items" {
			found = true
			assert.Equal(t, core.SeverityBlocking, is.Severity)
		}
	}
	assert.True(t, found, "4 items with Max 3 must be reported")
}

func TestValidate_UnavailableListIsWarning(t *testing.T) {
	// No TRACFIN field is bound to liste_ref_type_equipement.csv, so this is
	// covered on the hand-made spec, whose equipment list is Unavailable.
	spec := validationSpec()
	d := core.NewDocument()
	setValue(d, "organisme.equipement", "VALEUR-LIBRE")

	issues := core.Validate(spec, d)
	var warned bool
	for _, is := range issues {
		if is.Path != "organisme.equipement" {
			continue
		}
		assert.NotEqual(t, core.SeverityBlocking, is.Severity, "unavailable list must not block: %s", is.Code)
		if is.Code == "code_list_unavailable" {
			warned = true
			assert.Equal(t, core.SeverityWarning, is.Severity)
		}
	}
	assert.True(t, warned, "expected a code_list_unavailable warning")
}

func TestValidate_IssuesAreSortedByPathThenCode(t *testing.T) {
	spec := validationSpec()
	d := core.NewDocument()
	setValue(d, "organisme.profession", "ZZZ")
	setValue(d, "emetteur", "personnes.pp[pp-9]")
	d.AddItem("personnes.pp", "pp")
	d.AddItem("personnes.pp", "pp")
	d.AddItem("personnes.pp", "pp")
	d.AddItem("personnes.pp", "pp")

	issues := core.Validate(spec, d)
	require.NotEmpty(t, issues)
	for i := 1; i < len(issues); i++ {
		prev, cur := issues[i-1], issues[i]
		if prev.Path == cur.Path {
			assert.LessOrEqual(t, prev.Code, cur.Code)
		} else {
			assert.Less(t, string(prev.Path), string(cur.Path))
		}
	}
}

func TestIssue_Blocks(t *testing.T) {
	var none *core.Issue
	assert.False(t, none.Blocks())
	assert.True(t, (&core.Issue{Severity: core.SeverityBlocking}).Blocks())
	assert.False(t, (&core.Issue{Severity: core.SeverityWarning}).Blocks())
	assert.False(t, core.ValidateValue(validationSpec(), "organisme.equipement", "x").Blocks())
}

func TestValidate_EmptyRepeatableFieldItemDoesNotSatisfyRule(t *testing.T) {
	spec := &core.Spec{Root: &core.Node{Children: []*core.Node{
		{Name: "kind", Template: "kind", Kind: core.NodeField, Type: core.TypeString, Min: 0, Max: 1, Export: true},
		{Name: "alias", Template: "alias[]", Kind: core.NodeField, Type: core.TypeString, Min: 0, Max: 5, Export: true},
	}}, Rules: []core.Rule{{
		ID: "alias_requis", Kind: "required_if",
		When: &core.Condition{Path: "kind", Op: "eq", Values: []string{"X"}},
		Then: []string{"alias[]"},
	}}}
	d := core.NewDocument()
	setValue(d, "kind", "X")
	d.AddItem("alias", "alias") // empty item
	var fired bool
	for _, is := range core.Validate(spec, d) {
		fired = fired || is.Code == "rule:alias_requis"
	}
	assert.True(t, fired)

	setValue(d, "alias[alias-1]", "ok")
	for _, is := range core.Validate(spec, d) {
		assert.NotEqual(t, "rule:alias_requis", is.Code)
	}
}

func TestValidate_UnknownRuleAndNilSpec(t *testing.T) {
	assert.Nil(t, core.Validate(nil, core.NewDocument()))
	spec := validationSpec()
	spec.Rules = []core.Rule{{ID: "bad", Kind: "bogus"}, {ID: "badop", Kind: "required_if",
		When: &core.Condition{Path: "organisme.profession", Op: "weird"}, Then: []string{"organisme.libelle"}}}
	var n int
	for _, is := range core.Validate(spec, core.NewDocument()) {
		if is.Code == "unknown_rule" && is.Blocks() {
			n++
		}
	}
	assert.Equal(t, 2, n)
}
