package core_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const clearedPath = "personnes.pp[pp-1].etatCivil.lieuNaissance"

// A manual clear is kept as an empty value of manual origin: nothing computed
// or produced by the model brings the value back, and it reads as absent.
func TestApplyFills_ManualClearIsKeptAsTombstone(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	doc.Values[clearedPath] = core.Value{Value: "Paris", Origin: core.OriginLLM}

	res := core.ApplyFills(spec, doc, []core.Fill{{Path: clearedPath, Value: "", Origin: core.OriginManual}},
		core.ApplyOptions{Overwrite: core.OverwriteAll})
	require.Empty(t, res.Rejected)
	v, ok := doc.Values[clearedPath]
	require.True(t, ok, "the cleared field keeps a marker")
	assert.Equal(t, "", v.Value)
	assert.Equal(t, core.OriginManual, v.Origin)

	// A computation stays silent, a model output is a conflict; neither writes.
	res = core.ApplyFills(spec, doc, []core.Fill{{Path: clearedPath, Value: "Lyon", Origin: core.OriginComputed}},
		core.ApplyOptions{Overwrite: core.OverwriteNonManual})
	assert.Empty(t, res.Applied)
	assert.Empty(t, res.Conflicts)
	res = core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(clearedPath, "Lyon")},
		core.ApplyOptions{Overwrite: core.OverwriteNonManual})
	assert.Empty(t, res.Applied)
	require.Len(t, res.Conflicts, 1)
	assert.Equal(t, "", doc.Values[clearedPath].Value)

	// The analyst can fill it again, and the question resolver sees it as absent.
	res = core.ApplyFills(spec, doc, []core.Fill{{Path: clearedPath, Value: "Nice", Origin: core.OriginManual}},
		core.ApplyOptions{Overwrite: core.OverwriteAll})
	assert.Equal(t, "Nice", doc.Values[clearedPath].Value)
	assert.Len(t, res.Applied, 1)
}

func TestTombstone_ReadsAsAbsent(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	doc.Values[clearedPath] = core.Value{Origin: core.OriginManual}

	for _, i := range core.Validate(spec, doc) {
		assert.NotEqual(t, clearedPath, string(i.Path), "an empty marker raises no value issue: %s", i.Code)
	}
	issues := core.MarkReviewed(spec, doc, []core.Path{clearedPath})
	require.Len(t, issues, 1)
	assert.Equal(t, core.CodeNotFound, issues[0].Code)

	var buf bytes.Buffer
	_ = core.WriteXML(spec, doc, "ds", &buf)
	assert.NotContains(t, buf.String(), "lieuNaissance")

	clone := doc.Clone()
	assert.Contains(t, clone.Values, core.Path(clearedPath))
}

// A computed field cleared by hand stays empty while the computation gives the
// value it gave when cleared, and follows the computation again when the result
// changes.
func TestApplyFills_ClearedComputedFieldFollowsAChangedResult(t *testing.T) {
	spec := loadTracfinSpec(t)
	const total = core.Path("operations.montantTotal")
	computed := func(v string) core.Fill {
		return core.Fill{Path: total, Value: v, Origin: core.OriginComputed,
			Computation: &core.Computation{Name: "somme", Inputs: []core.Path{"t1"}}}
	}
	recompute := core.ApplyOptions{Overwrite: core.OverwriteNonManual}
	doc := core.NewDocument()
	require.Len(t, core.ApplyFills(spec, doc, []core.Fill{computed("1000")}, recompute).Applied, 1)

	core.ApplyFills(spec, doc, []core.Fill{{Path: total, Value: "", Origin: core.OriginManual}},
		core.ApplyOptions{Overwrite: core.OverwriteAll})
	v := doc.Values[total]
	require.Equal(t, "", v.Value)
	require.NotNil(t, v.Computation)
	assert.Equal(t, "1000", v.Computation.Result)

	res := core.ApplyFills(spec, doc, []core.Fill{computed("1000")}, recompute)
	assert.Empty(t, res.Applied, "same result: the field stays cleared")
	assert.Equal(t, "", doc.Values[total].Value)

	res = core.ApplyFills(spec, doc, []core.Fill{computed("1200")}, recompute)
	assert.Equal(t, []core.Path{total}, res.Applied, "another result: the field follows the computation")
	assert.Equal(t, "1200", doc.Values[total].Value)
	assert.Equal(t, core.OriginComputed, doc.Values[total].Origin)
}
