package core_test

import (
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// semanticsSpec: a repeatable `lines[]` block with one rule per operator,
// all scoped on the repeated template.
func semanticsSpec() *core.Spec {
	field := func(name string, typ core.FieldType) *core.Node {
		return &core.Node{Name: name, Template: "lines[]." + name, Kind: core.NodeField, Type: typ, Min: 0, Max: 1, Export: true}
	}
	scope := "lines[]"
	return &core.Spec{
		Key: core.FormatKey{Family: "test", Country: "FR", Version: "0"},
		Root: &core.Node{Export: true, Children: []*core.Node{
			{Name: "lines", Template: "lines[]", Kind: core.NodeGroup, Min: 0, Max: -1, Export: true, Children: []*core.Node{
				field("kind", core.TypeString), field("detail", core.TypeString), field("note", core.TypeString),
				field("flag", core.TypeString), field("extra", core.TypeString), field("alt", core.TypeString),
			}},
		}},
		Rules: []core.Rule{
			{ID: "r_eq", Kind: "required_if", Scope: scope, Then: []string{"lines[].detail"},
				When: &core.Condition{Path: "lines[].kind", Op: "eq", Values: []string{"X"}}},
			{ID: "r_not_in", Kind: "forbidden_if", Scope: scope, Then: []string{"lines[].note"},
				When: &core.Condition{Path: "lines[].kind", Op: "not_in", Values: []string{"A", "B"}}},
			{ID: "r_present", Kind: "required_if", Scope: scope, Then: []string{"lines[].extra"},
				When: &core.Condition{Path: "lines[].flag", Op: "present"}},
			{ID: "r_absent", Kind: "required_if", Scope: scope, Then: []string{"lines[].alt"},
				When: &core.Condition{Path: "lines[].flag", Op: "absent"}},
		},
	}
}

func ruleIssuePaths(issues []core.Issue, ruleID string) []core.Path {
	var out []core.Path
	for _, is := range issues {
		if is.RuleID == ruleID {
			out = append(out, is.Path)
		}
	}
	return out
}

func TestRules_EvaluatedPerItemNotAcrossItems(t *testing.T) {
	spec := semanticsSpec()
	d := core.NewDocument()
	l1 := d.AddItem("lines", "l")
	l2 := d.AddItem("lines", "l")
	setValue(d, l1.Child("kind"), "X") // requires detail, missing
	setValue(d, l2.Child("kind"), "X") // requires detail, present
	setValue(d, l2.Child("detail"), "ok")
	l3 := d.AddItem("lines", "l")
	setValue(d, l3.Child("kind"), "A") // does not trigger r_eq
	setValue(d, l3.Child("detail"), "x")

	issues := core.Validate(spec, d)
	assert.Equal(t, []core.Path{l1.Child("detail")}, ruleIssuePaths(issues, "r_eq"),
		"only the item whose own kind is X lacks its detail: a sibling's detail must not satisfy it")
}

func TestRules_NotInIsTrueWhenAbsentAndPerItem(t *testing.T) {
	spec := semanticsSpec()
	d := core.NewDocument()
	a := d.AddItem("lines", "l")
	b := d.AddItem("lines", "l")
	c := d.AddItem("lines", "l")
	setValue(d, a.Child("kind"), "A")
	setValue(d, a.Child("note"), "n") // kind in {A,B}: allowed
	setValue(d, b.Child("kind"), "Z")
	setValue(d, b.Child("note"), "n") // not in {A,B}: forbidden
	setValue(d, c.Child("note"), "n") // no kind at all: not_in is true -> forbidden

	got := ruleIssuePaths(core.Validate(spec, d), "r_not_in")
	assert.ElementsMatch(t, []core.Path{b.Child("note"), c.Child("note")}, got)
}

func TestRules_PresentAndAbsent(t *testing.T) {
	spec := semanticsSpec()
	d := core.NewDocument()
	withFlag := d.AddItem("lines", "l")
	withoutFlag := d.AddItem("lines", "l")
	setValue(d, withFlag.Child("flag"), "1")
	setValue(d, withoutFlag.Child("kind"), "A")

	issues := core.Validate(spec, d)
	assert.Equal(t, []core.Path{withFlag.Child("extra")}, ruleIssuePaths(issues, "r_present"))
	assert.Equal(t, []core.Path{withoutFlag.Child("alt")}, ruleIssuePaths(issues, "r_absent"))

	setValue(d, withFlag.Child("extra"), "e")
	setValue(d, withoutFlag.Child("alt"), "a")
	issues = core.Validate(spec, d)
	assert.Empty(t, ruleIssuePaths(issues, "r_present"))
	assert.Empty(t, ruleIssuePaths(issues, "r_absent"))
}

// facetSpec: code nodes with facets, a bad pattern, a repeatable field with a
// maximum, and a ref-or-text field.
func facetSpec() *core.Spec {
	spec := &core.Spec{
		Key: core.FormatKey{Family: "test", Country: "FR", Version: "0"},
		CodeLists: map[string]core.CodeList{
			"pays": {Name: "pays", Codes: []core.Code{{Value: "FR"}, {Value: "1"}}},
			"free": {Name: "free", Unavailable: true},
		},
		Root: &core.Node{Children: []*core.Node{
			{Name: "pays", Template: "pays", Kind: core.NodeField, Type: core.TypeCode, CodeList: "pays", MinLength: 2, MaxLength: 200, Min: 0, Max: 1},
			{Name: "free", Template: "free", Kind: core.NodeField, Type: core.TypeCode, CodeList: "free", MinLength: 2, Pattern: `[A-Z]+`, Min: 0, Max: 1},
			{Name: "broken", Template: "broken", Kind: core.NodeField, Type: core.TypeString, Pattern: `(`, Min: 0, Max: 1},
			{Name: "positive", Template: "positive", Kind: core.NodeField, Type: core.TypeInteger, MaxLength: 6, Pattern: `0*[1-9][0-9]*`, Min: 0, Max: 1},
			{Name: "amount", Template: "amount", Kind: core.NodeField, Type: core.TypeDecimal, MaxLength: 4, FractionDigits: 2, Min: 0, Max: 1},
			{Name: "tag", Template: "tag[]", Kind: core.NodeField, Type: core.TypeString, Min: 0, Max: 2},
			{Name: "person", Template: "person[]", Kind: core.NodeGroup, Min: 0, Max: -1, Children: []*core.Node{
				{Name: "name", Template: "person[].name", Kind: core.NodeField, Type: core.TypeString, Min: 0, Max: 1},
			}},
			{Name: "firm", Template: "firm[]", Kind: core.NodeGroup, Min: 0, Max: -1, Children: []*core.Node{
				{Name: "name", Template: "firm[].name", Kind: core.NodeField, Type: core.TypeString, Min: 0, Max: 1},
			}},
			{Name: "ref", Template: "ref", Kind: core.NodeField, Type: core.TypeRef, RefTargets: []string{"person[]"}, Min: 0, Max: 1},
			{Name: "reftext", Template: "reftext", Kind: core.NodeField, Type: core.TypeString, MaxLength: 200,
				RefOrText: true, RefTargets: []string{"person[]"}, Min: 0, Max: 1},
		}},
	}
	markExported(spec.Root)
	return spec
}

func TestValidateValue_CodeFacets(t *testing.T) {
	spec := facetSpec()
	issue := core.ValidateValue(spec, "pays", "1")
	require.NotNil(t, issue, "code 1 is in the list but shorter than the XSD minimum")
	assert.Equal(t, core.CodeMinLength, issue.Code)
	assert.True(t, issue.Blocks())
	assert.Nil(t, core.ValidateValue(spec, "pays", "FR"))

	// An unpublished list still enforces the facets, then warns.
	issue = core.ValidateValue(spec, "free", "ab")
	require.NotNil(t, issue)
	assert.Equal(t, core.CodePattern, issue.Code)
	issue = core.ValidateValue(spec, "free", "AB")
	require.NotNil(t, issue)
	assert.Equal(t, core.CodeCodeListUnavailable, issue.Code)
	assert.False(t, issue.Blocks())
}

func TestValidateValue_PositivePatternAndLeadingZeros(t *testing.T) {
	spec := facetSpec()
	for _, raw := range []string{"0", "000"} {
		issue := core.ValidateValue(spec, "positive", raw)
		require.NotNil(t, issue, raw)
		assert.Equal(t, core.CodePattern, issue.Code, raw)
	}
	assert.Nil(t, core.ValidateValue(spec, "positive", "7"))
	// totalDigits counts the value, not its leading zeros.
	assert.Nil(t, core.ValidateValue(spec, "positive", "0000000012"))
	assert.NotNil(t, core.ValidateValue(spec, "positive", "1234567"))
	assert.Nil(t, core.ValidateValue(spec, "amount", "0000012.50"), "3 significant digits, 1 fraction digit")
	assert.NotNil(t, core.ValidateValue(spec, "amount", "12345"))
	assert.NotNil(t, core.ValidateValue(spec, "amount", "1.234"))
}

func TestValidateValue_IllegalXMLCharsAreBlocking(t *testing.T) {
	spec := facetSpec()
	for _, raw := range []string{"\x01", "a\x00b", "￾", "\xff\xfe"} {
		issue := core.ValidateValue(spec, "reftext", raw)
		require.NotNil(t, issue, "%q", raw)
		assert.Equal(t, core.CodeBadChar, issue.Code, "%q", raw)
		assert.True(t, issue.Blocks())
	}
	assert.Nil(t, core.ValidateValue(spec, "reftext", "tab\tnewline\nok é"))
}

func TestValidateValue_UnreadablePatternBlocks(t *testing.T) {
	issue := core.ValidateValue(facetSpec(), "broken", "anything")
	require.NotNil(t, issue)
	assert.Equal(t, core.CodeBadPattern, issue.Code)
	assert.True(t, issue.Blocks())
}

func TestValidate_WrongKindRefOrTextIsBadRef(t *testing.T) {
	spec := facetSpec()
	d := core.NewDocument()
	p := d.AddItem("person", "p")
	f := d.AddItem("firm", "f")

	// Right kind: accepted. Free text: accepted. Wrong kind (existing or not): bad_ref.
	setValue(d, "reftext", string(p))
	assert.Empty(t, blockingIssues(core.Validate(spec, d)))
	setValue(d, "reftext", "plain text")
	assert.Empty(t, blockingIssues(core.Validate(spec, d)))
	for _, raw := range []string{string(f), "firm[f-99]"} {
		setValue(d, "reftext", raw)
		issues := blockingIssues(core.Validate(spec, d))
		require.Len(t, issues, 1, raw)
		assert.Equal(t, core.CodeBadRef, issues[0].Code, raw)
	}
	setValue(d, "reftext", "plain text")

	// TypeRef: wrong kind is bad_ref too.
	setValue(d, "ref", string(f))
	issues := blockingIssues(core.Validate(spec, d))
	require.Len(t, issues, 1)
	assert.Equal(t, core.CodeBadRef, issues[0].Code)
}

func TestValidate_TooManyItemsIgnoresEmptyFieldItems(t *testing.T) {
	spec := facetSpec()
	d := core.NewDocument()
	for i := 0; i < 4; i++ {
		item := d.AddItem("tag", "t")
		if i < 2 {
			setValue(d, item, "v")
		}
	}
	assert.Empty(t, blockingIssues(core.Validate(spec, d)), "two filled items, two empty ones (not exported): within the maximum of 2")

	setValue(d, "tag[t-3]", "w")
	issues := blockingIssues(core.Validate(spec, d))
	require.Len(t, issues, 1)
	assert.Equal(t, core.CodeTooManyItems, issues[0].Code)
}
