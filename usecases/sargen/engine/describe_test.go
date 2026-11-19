package engine

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// The part description carries what field_spec and search_code give.

func describeCodes(prefix string, n int) []core.Code {
	codes := make([]core.Code, n)
	for i := range codes {
		codes[i] = core.Code{Value: fmt.Sprintf("%s%02d", prefix, i), Label: fmt.Sprintf("Label %s %d", prefix, i)}
	}
	return codes
}

func describeField(name string, mutate func(n *core.Node)) *core.Node {
	n := &core.Node{
		Name: name, Template: "g." + name, Kind: core.NodeField, Min: 0, Max: 1, Export: true,
		Type: core.TypeString, Nature: core.NatureCopied,
	}
	if mutate != nil {
		mutate(n)
	}
	return n
}

func describeSpec(lists map[string]core.CodeList, fields ...*core.Node) *core.Spec {
	g := &core.Node{Name: "g", Template: "g", Kind: core.NodeGroup, Min: 1, Max: 1, Export: true, Children: fields}
	return &core.Spec{Root: &core.Node{Kind: core.NodeGroup, Children: []*core.Node{g}}, CodeLists: lists}
}

func TestDescribeSubtree_FieldCarriesFieldSpecDetail(t *testing.T) {
	long := strings.Repeat("é", 300)
	spec := describeSpec(nil,
		describeField("ref", func(n *core.Node) {
			n.Min, n.MinLength, n.MaxLength = 1, 2, 10
			n.Pattern = `[A-Z]{2}[0-9]+`
			n.Description = long
		}),
		describeField("amount", func(n *core.Node) { n.Type, n.FractionDigits = core.TypeDecimal, 2 }),
	)
	text := DescribeSubtree(spec, core.NewDocument(), []core.Path{"g"})

	ref := agLineWith(text, "g.ref")
	require.NotEmpty(t, ref, text)
	assert.Contains(t, ref, "required")
	assert.Contains(t, ref, "length 2 to 10")
	assert.Contains(t, ref, `pattern [A-Z]{2}[0-9]+`)
	assert.Contains(t, ref, strings.Repeat("é", 200)+"…", "the description is trimmed to 200 runes")
	assert.NotContains(t, ref, strings.Repeat("é", 201))

	amount := agLineWith(text, "g.amount")
	assert.Contains(t, amount, "optional")
	assert.Contains(t, amount, "decimal")
	assert.Contains(t, amount, "2 decimals")
}

func TestDescribeSubtree_ShortCodeListIsInlinedOnce(t *testing.T) {
	spec := describeSpec(
		map[string]core.CodeList{"small.csv": {Name: "small.csv", Codes: describeCodes("S", 60), UnknownCode: "S00"}},
		describeField("a", func(n *core.Node) { n.Type, n.CodeList = core.TypeCode, "small.csv" }),
		describeField("b", func(n *core.Node) { n.Type, n.CodeList = core.TypeCode, "small.csv" }),
	)
	text := DescribeSubtree(spec, core.NewDocument(), []core.Path{"g"})

	assert.Equal(t, 1, strings.Count(text, "S17 = Label S 17"), "each code is written once, as `code = label`")
	assert.Equal(t, 1, strings.Count(text, "small.csv (60 codes)"))
	assert.Contains(t, text, "### Code lists")
	assert.Contains(t, text, `unknown code "S00": use it when the data say nothing`)
	assert.Contains(t, agLineWith(text, "g.a"), "code list: small.csv")
	assert.Contains(t, agLineWith(text, "g.b"), "code list: small.csv")
}

func TestDescribeSubtree_LongCodeListIsNamedWithItsSize(t *testing.T) {
	spec := describeSpec(
		map[string]core.CodeList{"big.csv": {Name: "big.csv", Codes: describeCodes("B", 61)}},
		describeField("a", func(n *core.Node) { n.Type, n.CodeList = core.TypeCode, "big.csv" }),
	)
	text := DescribeSubtree(spec, core.NewDocument(), []core.Path{"g"})

	assert.Contains(t, text, "big.csv (61 codes)")
	assert.Contains(t, text, "search_code")
	assert.NotContains(t, text, "B05 = ", "the codes of a long list are not written")
}

func TestDescribeSubtree_BoundIsHardOnFieldsAndCodeLists(t *testing.T) {
	var fields []*core.Node
	for i := 0; i < 40; i++ {
		fields = append(fields, describeField(fmt.Sprintf("f%02d", i), func(n *core.Node) {
			n.MinLength, n.MaxLength, n.Pattern = 1, 30, "[a-z]+"
			n.Description = strings.Repeat("d", 150)
		}))
	}
	spec := describeSpec(nil, fields...)
	doc := core.NewDocument()

	full := describeSubtree(spec, doc, []core.Path{"g"}, describeMaxBytes)
	assert.Equal(t, 40, strings.Count(full, "length 1 to 30"), "within the bound every template has the full form")

	const limit = 3000
	text := describeSubtree(spec, doc, []core.Path{"g"}, limit)
	assert.LessOrEqual(t, len(text), limit+200, "fields and code lists respect the bound")
	assert.Less(t, strings.Count(text, "length 1 to 30"), 40, "past the bound the description is dropped")
	assert.Contains(t, agLineWith(text, "g.f00"), "pattern", "the first fields keep the full form")
	assert.Contains(t, text, "more fields not described here: use field_spec", "fields that do not fit are counted")
}

func TestDescribeSubtree_ItemsShareTheirTemplate(t *testing.T) {
	spec := describeSpec(nil,
		describeField("ref", func(n *core.Node) { n.Pattern = "[A-Z]{3}"; n.Description = "reference of the line" }),
		describeField("amount", func(n *core.Node) { n.Type = core.TypeDecimal }),
	)
	// Make the group repeatable: items keyed by the document.
	spec.Root.Children[0].Max = 5
	for _, c := range spec.Root.Children[0].Children {
		c.Template = "g[]." + c.Name
	}
	spec.Root.Children[0].Template = "g[]"
	doc := core.NewDocument()
	var allowed []core.Path
	for i := 0; i < 10; i++ {
		k := doc.AddItem("g", "g")
		allowed = append(allowed, k)
	}
	doc.Values[core.Path(string(allowed[3])+".ref")] = core.Value{Value: "ABC"}

	text := DescribeSubtree(spec, doc, allowed)

	assert.Equal(t, 1, strings.Count(text, "pattern [A-Z]{3}"), "the detail of a field is written once for all the items")
	assert.Equal(t, 1, strings.Count(text, "reference of the line"))
	assert.Equal(t, 1, strings.Count(text, "- g[]."+"ref"))
	for _, k := range allowed {
		assert.Contains(t, text, "- "+string(k)+" — block", "every item is listed by its key")
	}
	assert.Contains(t, text, string(allowed[3])+".ref ; current value: ")
	assert.Equal(t, 1, strings.Count(text, "current value: "), "only valued fields are listed")
}

func TestDescribeSubtree_RepeatableFieldItemsReadEmpty(t *testing.T) {
	spec := describeSpec(nil, describeField("tags", func(n *core.Node) { n.Max = 5; n.Template = "g.tags[]" }))
	doc := core.NewDocument()
	k := doc.AddItem("g.tags", "tags")
	text := DescribeSubtree(spec, doc, []core.Path{"g"})
	assert.Contains(t, text, "- "+string(k)+" ; (empty)")
}

func TestDescribeSubtree_LateShortListStillHasItsEntry(t *testing.T) {
	var fields []*core.Node
	for i := 0; i < 30; i++ {
		fields = append(fields, describeField(fmt.Sprintf("f%02d", i), nil))
	}
	fields = append(fields, describeField("late", func(n *core.Node) { n.Type, n.CodeList = core.TypeCode, "mid.csv" }))
	spec := describeSpec(map[string]core.CodeList{"mid.csv": {Name: "mid.csv", Codes: describeCodes("M", 45)}}, fields...)
	text := DescribeSubtree(spec, core.NewDocument(), []core.Path{"g"})
	assert.Contains(t, text, "mid.csv (45 codes)")
	assert.Contains(t, text, "M44 = Label M 44")
	assert.NotContains(t, agLineWith(text, "g.late"), "M01", "the field line only names its list")
	assert.NotContains(t, agLineWith(text, "g.late"), "Label M")
}

func TestDescribeSubtree_RealSpecLotOfTenTransactions(t *testing.T) {
	tc := agentToolContext(t)
	var allowed []core.Path
	for i := 0; i < 10; i++ {
		allowed = append(allowed, tc.Doc.AddItem("work.transactions", "transaction"))
	}
	text := DescribeSubtree(tc.Format.Spec(), tc.Doc, allowed)

	assert.LessOrEqual(t, len(text), 12*1024, "ten transactions: templates once, items by key")
	fieldLines := 0
	seen := map[string]int{}
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "- work.transactions[].") {
			seen[strings.SplitN(l, " ", 3)[1]]++
			fieldLines++
		}
	}
	require.NotZero(t, fieldLines)
	for tmpl, n := range seen {
		assert.Equal(t, 1, n, "template %s written once", tmpl)
	}
	for _, k := range allowed {
		assert.Contains(t, text, "- "+string(k)+" — block")
	}
}

func TestDescribeSubtree_DeterministicUpToTheNonce(t *testing.T) {
	tc := agentToolContext(t)
	tc.Doc.Values[agNomPath] = core.Value{Value: "Durand", Origin: core.OriginManual}
	allowed := []core.Path{agentPersonPath, "context.general.refInterne"}
	nonce := regexp.MustCompile(`id="[0-9a-f]+"`)
	norm := func() string {
		return nonce.ReplaceAllString(DescribeSubtree(tc.Format.Spec(), tc.Doc, allowed), `id="N"`)
	}
	first := norm()
	for i := 0; i < 5; i++ {
		require.Equal(t, first, norm())
	}
	one := DescribeSubtree(tc.Format.Spec(), tc.Doc, allowed)
	assert.Equal(t, 2, strings.Count(one, `id="`), "one nonce per description: opening and closing tag share it")
}
