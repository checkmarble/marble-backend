package engine

import (
	"fmt"
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

func TestDescribeSubtree_BoundFallsBackToTheShortForm(t *testing.T) {
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
	assert.NotContains(t, full, "max length 30 ; nature", "within the bound every field has the full form")
	assert.Equal(t, 40, strings.Count(full, "length 1 to 30"))

	const limit = 3000
	text := describeSubtree(spec, doc, []core.Path{"g"}, limit)
	assert.Equal(t, 40, strings.Count(text, "\n- g.f")+1, "every field is still described")
	assert.Less(t, strings.Count(text, "length 1 to 30"), 40, "past the bound the full form stops")
	assert.NotEmpty(t, agLineWith(text, "g.f39"))
	assert.NotContains(t, agLineWith(text, "g.f39"), "pattern", "the last field is in the short form")
	assert.Contains(t, agLineWith(text, "g.f39"), "max length 30")
	assert.Contains(t, agLineWith(text, "g.f00"), "pattern", "the first fields keep the full form")
	// The full-form lines respect the bound: only short lines come after it.
	assert.LessOrEqual(t, strings.LastIndex(text, "length 1 to 30"), limit)
}

func TestDescribeSubtree_Deterministic(t *testing.T) {
	tc := agentToolContext(t)
	allowed := []core.Path{agentPersonPath, "context.general.refInterne"}
	first := DescribeSubtree(tc.Format.Spec(), tc.Doc, allowed)
	for i := 0; i < 5; i++ {
		require.Equal(t, first, DescribeSubtree(tc.Format.Spec(), tc.Doc, allowed))
	}
}
