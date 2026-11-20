package core_test

import (
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyFills_ManualValueIsReviewed(t *testing.T) {
	spec := loadTracfinSpec(t)
	path := mustParsePath(t, pp1Name)
	for _, origin := range []core.Origin{core.OriginManual, core.OriginAnalyst} {
		t.Run("non-empty "+string(origin), func(t *testing.T) {
			doc, _ := docWithPerson(t)
			res := core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "Martin", Origin: origin}},
				core.ApplyOptions{Overwrite: core.OverwriteAll})
			require.Empty(t, res.Rejected)
			assert.Equal(t, []core.Path{path}, res.Applied)
			assert.True(t, doc.Values[path].Reviewed)
			assert.Equal(t, origin, doc.Values[path].Origin)
		})
	}
	t.Run("a non-manual fill is not reviewed", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(string(path), "Martin")},
			core.ApplyOptions{Overwrite: core.OverwriteAll})
		require.Empty(t, res.Rejected)
		assert.False(t, doc.Values[path].Reviewed)
	})
	t.Run("manual clear is not a reviewed value", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		doc.Values[path] = core.Value{Value: "Dupont", Origin: core.OriginLLM}
		core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "", Origin: core.OriginManual}},
			core.ApplyOptions{Overwrite: core.OverwriteAll})
		assert.Empty(t, doc.Values[path].Value)
		assert.False(t, doc.Values[path].Reviewed)
	})
	t.Run("overwriting a computed value leaves it manual and reviewed", func(t *testing.T) {
		doc := core.NewDocument()
		p := core.Path(total)
		doc.Values[p] = core.Value{Value: "100", Origin: core.OriginComputed,
			Computation: &core.Computation{Name: "sum", Inputs: []core.Path{"a"}}}
		core.ApplyFills(spec, doc, []core.Fill{{Path: p, Value: "250", Origin: core.OriginManual}},
			core.ApplyOptions{Overwrite: core.OverwriteAll})
		v := doc.Values[p]
		assert.Equal(t, core.OriginManual, v.Origin)
		assert.True(t, v.Reviewed)
		assert.Equal(t, "250", v.Value)
	})
}

func TestApplyFills_RecomputeRefreshesComputation(t *testing.T) {
	spec := loadTracfinSpec(t)
	p := core.Path(total)
	computed := func(inputs ...core.Path) core.Fill {
		return core.Fill{Path: p, Value: "100", Origin: core.OriginComputed,
			Computation: &core.Computation{Name: "sum", Inputs: inputs}}
	}
	opts := core.ApplyOptions{Overwrite: core.OverwriteNonManual}

	t.Run("same value, other inputs: computation is refreshed, nothing else", func(t *testing.T) {
		doc := core.NewDocument()
		doc.Values[p] = core.Value{Value: "100", Origin: core.OriginComputed, Reviewed: true,
			Justification: "kept", Computation: &core.Computation{Name: "sum", Inputs: []core.Path{"t1", "t2"}}}
		res := core.ApplyFills(spec, doc, []core.Fill{computed("t2", "t3")}, opts)
		assert.Empty(t, res.Applied, "not an applied write")
		assert.Equal(t, []core.Path{p}, res.Refreshed)
		assert.Empty(t, res.Conflicts)
		v := doc.Values[p]
		assert.Equal(t, []core.Path{"t2", "t3"}, v.Computation.Inputs)
		assert.Equal(t, "100", v.Value)
		assert.True(t, v.Reviewed, "reviewed flag kept")
		assert.Equal(t, "kept", v.Justification)
	})
	t.Run("same computation: nothing happens", func(t *testing.T) {
		doc := core.NewDocument()
		doc.Values[p] = core.Value{Value: "100", Origin: core.OriginComputed,
			Computation: &core.Computation{Name: "sum", Inputs: []core.Path{"t1"}}}
		res := core.ApplyFills(spec, doc, []core.Fill{computed("t1")}, opts)
		assert.Empty(t, res.Applied)
		assert.Empty(t, res.Refreshed)
	})
	t.Run("a value of another origin is untouched", func(t *testing.T) {
		for _, o := range []core.Origin{core.OriginManual, core.OriginLLM} {
			doc := core.NewDocument()
			stored := core.Value{Value: "100", Origin: o, Computation: &core.Computation{Name: "sum", Inputs: []core.Path{"t1"}}}
			doc.Values[p] = stored
			res := core.ApplyFills(spec, doc, []core.Fill{computed("t9")}, opts)
			assert.Empty(t, res.Refreshed)
			assert.Equal(t, stored, doc.Values[p])
		}
	})
}
