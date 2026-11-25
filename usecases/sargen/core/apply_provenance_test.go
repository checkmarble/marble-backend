package core_test

import (
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ApplyFills provenance: overwrite policies, manual values, corrections of
// computed values, recompute and the reviewed flag.

func TestApplyFills_ManualValueIsNotReviewed(t *testing.T) {
	spec := loadTracfinSpec(t)
	path := mustParsePath(t, pp1Name)
	t.Run("typing a value is not reviewing it", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		res := core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "Martin", Origin: core.OriginManual}},
			core.ApplyOptions{Overwrite: core.OverwriteAll})
		require.Empty(t, res.Rejected)
		assert.Equal(t, []core.Path{path}, res.Applied)
		assert.False(t, doc.Values[path].Reviewed)
		assert.Equal(t, core.OriginManual, doc.Values[path].Origin)
	})
	t.Run("Fill.Reviewed is honoured", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "Martin", Origin: core.OriginManual, Reviewed: true}},
			core.ApplyOptions{Overwrite: core.OverwriteAll})
		assert.True(t, doc.Values[path].Reviewed)
	})
	t.Run("retyping a stored manual value loses nothing", func(t *testing.T) {
		for _, policy := range []core.OverwritePolicy{core.OverwriteNonManual, core.OverwriteAll} {
			doc, _ := docWithPerson(t)
			stored := core.Value{Value: "Dupont", Origin: core.OriginManual, Reviewed: true,
				Sources:       []core.SourceRef{llmSource("p", "Dupont")},
				Justification: "kept", Confidence: core.ConfidenceHigh}
			doc.Values[path] = stored
			res := core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "Dupont", Origin: core.OriginManual}},
				core.ApplyOptions{Overwrite: policy})
			require.Empty(t, res.Rejected)
			assert.Empty(t, res.Conflicts)
			assert.Equal(t, stored, doc.Values[path], "policy %q", policy)
		}
	})
	t.Run("manual clear deletes the value", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		doc.Values[path] = core.Value{Value: "Dupont", Origin: core.OriginLLM}
		core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "", Origin: core.OriginManual}},
			core.ApplyOptions{Overwrite: core.OverwriteAll})
		assert.Empty(t, doc.Values[path].Value)
		assert.False(t, doc.Values[path].Reviewed)
	})
}

// Retyping the stored value is not a review: nothing changes. Reviewing is
// MarkReviewed, which keeps the origin and the provenance.
func TestApplyFills_RetypingTheStoredValueIsANoOp(t *testing.T) {
	spec := loadTracfinSpec(t)
	retype := func(path core.Path, value string) core.Fill {
		return core.Fill{Path: path, Value: value, Origin: core.OriginManual}
	}
	t.Run("llm value", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		path := mustParsePath(t, pp1Name)
		stored := core.Value{Value: "Dupont", Origin: core.OriginLLM, Sources: []core.SourceRef{llmSource("p", "Dupont")},
			Justification: "nom relevé sur la pièce", Confidence: core.ConfidenceHigh}
		doc.Values[path] = stored
		res := core.ApplyFills(spec, doc, []core.Fill{retype(path, "Dupont")}, core.ApplyOptions{Overwrite: core.OverwriteAll})
		assert.Empty(t, res.Applied)
		assert.Empty(t, res.Rejected)
		assert.Equal(t, stored, doc.Values[path])

		require.Empty(t, core.MarkReviewed(spec, doc, []core.Path{path}))
		got := doc.Values[path]
		assert.True(t, got.Reviewed)
		assert.Equal(t, core.OriginLLM, got.Origin, "a review keeps the origin")
		assert.Equal(t, stored.Sources, got.Sources)
	})
	t.Run("computed total", func(t *testing.T) {
		doc := core.NewDocument()
		path := core.Path(total)
		stored := core.Value{Value: "1000", Origin: core.OriginComputed,
			Computation: &core.Computation{Name: "somme", Inputs: []core.Path{"t1"}}}
		doc.Values[path] = stored
		res := core.ApplyFills(spec, doc, []core.Fill{retype(path, "1000")}, core.ApplyOptions{Overwrite: core.OverwriteAll})
		assert.Empty(t, res.Applied)
		assert.Equal(t, stored, doc.Values[path], "still computed: it keeps following its inputs")
	})
	t.Run("differing value overwrites", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		path := mustParsePath(t, pp1Name)
		doc.Values[path] = core.Value{Value: "Dupont", Origin: core.OriginLLM, Sources: []core.SourceRef{llmSource("p", "Dupont")}}
		res := core.ApplyFills(spec, doc, []core.Fill{retype(path, "Martin")}, core.ApplyOptions{Overwrite: core.OverwriteAll})
		assert.Equal(t, []core.Path{path}, res.Applied)
		got := doc.Values[path]
		assert.Equal(t, "Martin", got.Value)
		assert.Equal(t, core.OriginManual, got.Origin)
		assert.Empty(t, got.Sources)
	})
}

// A computed field corrected by hand keeps its correction while the
// computation gives the value it gave at correction time, and follows the
// computation again when the result changes.
func TestApplyFills_ManualCorrectionOfAComputedValue(t *testing.T) {
	spec := loadTracfinSpec(t)
	path := core.Path(total)
	recompute := core.ApplyOptions{Overwrite: core.OverwriteNonManual}
	computed := func(v string) core.Fill {
		return core.Fill{Path: path, Value: v, Origin: core.OriginComputed,
			Computation: &core.Computation{Name: "somme", Inputs: []core.Path{"t1"}}}
	}
	corrected := func(t *testing.T) *core.Document {
		t.Helper()
		doc := core.NewDocument()
		doc.Values[path] = core.Value{Value: "1000", Origin: core.OriginComputed, Reviewed: true,
			Computation: &core.Computation{Name: "somme", Inputs: []core.Path{"t1"}}}
		res := core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "950", Origin: core.OriginManual}},
			core.ApplyOptions{Overwrite: core.OverwriteAll})
		require.Equal(t, []core.Path{path}, res.Applied)
		return doc
	}

	t.Run("the correction keeps the computation and its result", func(t *testing.T) {
		v := corrected(t).Values[path]
		assert.Equal(t, "950", v.Value)
		assert.Equal(t, core.OriginManual, v.Origin)
		assert.False(t, v.Reviewed)
		require.NotNil(t, v.Computation)
		assert.Equal(t, "somme", v.Computation.Name)
		assert.Equal(t, "1000", v.Computation.Result)
	})
	t.Run("same computed result: the correction stays", func(t *testing.T) {
		doc := corrected(t)
		before := doc.Values[path]
		res := core.ApplyFills(spec, doc, []core.Fill{computed("1000")}, recompute)
		assert.Empty(t, res.Applied)
		assert.Empty(t, res.Conflicts)
		assert.Equal(t, before, doc.Values[path])
	})
	t.Run("same result from other inputs replaces the correction", func(t *testing.T) {
		doc := corrected(t)
		f := computed("1000")
		f.Computation.Inputs = []core.Path{"t1", "t2"}
		res := core.ApplyFills(spec, doc, []core.Fill{f}, recompute)
		assert.Equal(t, []core.Path{path}, res.Applied, "a transaction added: not the same data any more")
		v := doc.Values[path]
		assert.Equal(t, "1000", v.Value)
		assert.Equal(t, core.OriginComputed, v.Origin)
		assert.Equal(t, []core.Path{"t1", "t2"}, v.Computation.Inputs)
	})
	t.Run("same inputs in another order keep the correction", func(t *testing.T) {
		doc := core.NewDocument()
		doc.Values[path] = core.Value{Value: "1000", Origin: core.OriginComputed,
			Computation: &core.Computation{Name: "somme", Inputs: []core.Path{"t1", "t2"}}}
		core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "950", Origin: core.OriginManual}},
			core.ApplyOptions{Overwrite: core.OverwriteAll})
		f := computed("1000")
		f.Computation.Inputs = []core.Path{"t2", "t1"}
		res := core.ApplyFills(spec, doc, []core.Fill{f}, recompute)
		assert.Empty(t, res.Applied)
		assert.Equal(t, "950", doc.Values[path].Value)
	})
	t.Run("another computed result replaces the correction, unreviewed", func(t *testing.T) {
		doc := corrected(t)
		require.Empty(t, core.MarkReviewed(spec, doc, []core.Path{path}))
		res := core.ApplyFills(spec, doc, []core.Fill{computed("1200")}, recompute)
		assert.Equal(t, []core.Path{path}, res.Applied)
		v := doc.Values[path]
		assert.Equal(t, "1200", v.Value)
		assert.Equal(t, core.OriginComputed, v.Origin)
		assert.False(t, v.Reviewed)
		assert.Empty(t, v.Computation.Result)
	})
	t.Run("a second correction keeps the first computed result", func(t *testing.T) {
		doc := corrected(t)
		core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "900", Origin: core.OriginManual}},
			core.ApplyOptions{Overwrite: core.OverwriteAll})
		assert.Equal(t, "1000", doc.Values[path].Computation.Result)
		res := core.ApplyFills(spec, doc, []core.Fill{computed("1000")}, recompute)
		assert.Empty(t, res.Applied)
		assert.Equal(t, "900", doc.Values[path].Value)
	})
	t.Run("a value typed with nothing computed gives way to the first computed value", func(t *testing.T) {
		doc := core.NewDocument()
		core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "500", Origin: core.OriginManual}},
			core.ApplyOptions{Overwrite: core.OverwriteAll})
		assert.Nil(t, doc.Values[path].Computation)
		res := core.ApplyFills(spec, doc, []core.Fill{computed("700")}, recompute)
		assert.Equal(t, []core.Path{path}, res.Applied)
		assert.Equal(t, core.OriginComputed, doc.Values[path].Origin)
	})
	t.Run("never policy still protects the correction", func(t *testing.T) {
		doc := corrected(t)
		res := core.ApplyFills(spec, doc, []core.Fill{computed("1200")}, core.ApplyOptions{})
		assert.Len(t, res.Conflicts, 1)
		assert.Equal(t, "950", doc.Values[path].Value)
	})
}

func TestApplyFills_SameValueIsANoOpWhateverTheOrigin(t *testing.T) {
	spec := loadTracfinSpec(t)
	path := mustParsePath(t, pp1Name)
	for _, o := range []core.Origin{core.OriginLLM, core.OriginComputed, core.OriginManual} {
		doc, _ := docWithPerson(t)
		doc.Values[path] = core.Value{Value: "Dupont", Origin: core.OriginManual}
		f := llmCopiedFill(string(path), "Dupont")
		f.Origin = o
		res := core.ApplyFills(spec, doc, []core.Fill{f}, core.ApplyOptions{Overwrite: core.OverwriteAll})
		assert.Empty(t, res.Applied, string(o))
		assert.Equal(t, core.OriginManual, doc.Values[path].Origin)
	}
}

// Organization settings are written only from the organization settings or by
// the analyst; the settings origin writes only settings.
func TestApplyFills_SettingsOrigins(t *testing.T) {
	spec := loadTracfinSpec(t)
	const setting = "organisme.libelle"
	opts := core.ApplyOptions{Overwrite: core.OverwriteAll}
	for _, o := range []core.Origin{core.OriginOrgSetting, core.OriginManual} {
		doc := core.NewDocument()
		res := core.ApplyFills(spec, doc, []core.Fill{{Path: setting, Value: "Banque", Origin: o}}, opts)
		assert.Empty(t, res.Rejected, string(o))
		assert.Equal(t, "Banque", valueOf(doc, setting))
	}
	t.Run("a computed fill cannot write a setting", func(t *testing.T) {
		doc := core.NewDocument()
		res := core.ApplyFills(spec, doc, []core.Fill{{Path: setting, Value: "Banque", Origin: core.OriginComputed}}, opts)
		assert.Equal(t, []string{core.CodeOriginNotAllowed}, rejectedCodes(res))
		assert.Equal(t, "computed", res.Rejected[0].Issue.Params["origin"])
	})
	t.Run("the settings origin cannot write another field", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		res := core.ApplyFills(spec, doc, []core.Fill{{Path: pp1Name, Value: "Dupont", Origin: core.OriginOrgSetting}}, opts)
		assert.Equal(t, []string{core.CodeOriginNotAllowed}, rejectedCodes(res))
	})
}

func TestApplyFills_OverwritePolicies(t *testing.T) {
	spec := loadTracfinSpec(t)
	path := mustParsePath(t, pp1Name)
	existing := func(o core.Origin) *core.Document {
		doc, _ := docWithPerson(t)
		doc.Values[path] = core.Value{Value: "Dupont", Origin: o}
		return doc
	}

	t.Run("never: non-empty value is a conflict", func(t *testing.T) {
		doc := existing(core.OriginLLM)
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(string(path), "Martin")}, core.ApplyOptions{})
		assert.Len(t, res.Conflicts, 1)
		assert.Equal(t, "Dupont", valueOf(doc, path))
	})
	t.Run("never: empty slot is written", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(string(path), "Martin")}, core.ApplyOptions{})
		assert.Equal(t, []core.Path{path}, res.Applied)
	})
	t.Run("all: overwrites a manual value", func(t *testing.T) {
		doc := existing(core.OriginManual)
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(string(path), "Martin")}, core.ApplyOptions{Overwrite: core.OverwriteAll})
		assert.Empty(t, res.Conflicts)
		assert.Equal(t, "Martin", valueOf(doc, path))
	})
	t.Run("non manual: manual origin is protected", func(t *testing.T) {
		doc := existing(core.OriginManual)
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(string(path), "Martin")}, core.ApplyOptions{Overwrite: core.OverwriteNonManual})
		assert.Len(t, res.Conflicts, 1)
		assert.Equal(t, "Dupont", valueOf(doc, path))
	})
	t.Run("non manual: same LLM value is a silent no-op", func(t *testing.T) {
		doc := existing(core.OriginLLM)
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(string(path), "Dupont")}, core.ApplyOptions{Overwrite: core.OverwriteNonManual})
		assert.Empty(t, res.Conflicts)
		assert.Empty(t, res.Rejected)
		assert.Empty(t, res.Applied)
	})
}

func TestApplyFills_NonManualReplacesLLMAndComputed(t *testing.T) {
	spec := loadTracfinSpec(t)
	path := mustParsePath(t, pp1Name)
	for _, o := range []core.Origin{core.OriginLLM, core.OriginComputed} {
		doc, _ := docWithPerson(t)
		doc.Values[path] = core.Value{Value: "Dupont", Origin: o}
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(string(path), "Martin")}, core.ApplyOptions{Overwrite: core.OverwriteNonManual})
		assert.Empty(t, res.Conflicts, string(o))
		assert.Equal(t, []core.Path{path}, res.Applied)
		assert.Equal(t, "Martin", doc.Values[path].Value)
	}
}

func TestApplyFills_ComputedRecompute(t *testing.T) {
	spec := loadTracfinSpec(t)
	path := core.Path(total)
	computed := func(v string) core.Fill {
		return core.Fill{Path: path, Value: v, Origin: core.OriginComputed}
	}
	t.Run("reviewed computed value is recomputed unreviewed", func(t *testing.T) {
		doc := core.NewDocument()
		doc.Values[path] = core.Value{Value: "100", Origin: core.OriginComputed, Reviewed: true}
		res := core.ApplyFills(spec, doc, []core.Fill{computed("200")}, core.ApplyOptions{Overwrite: core.OverwriteNonManual})
		require.Empty(t, res.Conflicts)
		require.Empty(t, res.Rejected)
		assert.Equal(t, []core.Path{path}, res.Applied)
		assert.Equal(t, "200", doc.Values[path].Value)
		assert.False(t, doc.Values[path].Reviewed)
	})
	t.Run("analyst value on a non-computed field stays protected from computed", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		name := mustParsePath(t, pp1Name)
		doc.Values[name] = core.Value{Value: "Dupont", Origin: core.OriginManual}
		res := core.ApplyFills(spec, doc, []core.Fill{{Path: name, Value: "Martin", Origin: core.OriginComputed}},
			core.ApplyOptions{Overwrite: core.OverwriteNonManual})
		assert.Len(t, res.Conflicts, 1)
		assert.Equal(t, "Dupont", doc.Values[name].Value)
	})
	t.Run("reviewed LLM value vs LLM fill conflicts", func(t *testing.T) {
		doc := core.NewDocument()
		name := mustParsePath(t, "personnes.pp[pp-1].etatCivil.nomNaissance")
		doc.AddItem("personnes.pp", "pp")
		doc.Values[name] = core.Value{Value: "Dupont", Origin: core.OriginLLM, Reviewed: true}
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(string(name), "Martin")}, core.ApplyOptions{Overwrite: core.OverwriteNonManual})
		assert.Len(t, res.Conflicts, 1)
		assert.Equal(t, "Dupont", doc.Values[name].Value)
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

func TestMarkReviewedAndFillReviewed(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	name := mustParsePath(t, pp1Name)
	res := core.ApplyFills(spec, doc, []core.Fill{
		llmCopiedFill(string(name), "Dupont"),
		{Path: core.Path(prenoms + "[+a]"), Value: "Jean", Origin: core.OriginManual, Reviewed: true},
	}, core.ApplyOptions{})
	require.Empty(t, res.Rejected)
	assert.False(t, doc.Values[name].Reviewed)
	assert.True(t, doc.Values[res.Applied[1]].Reviewed, "Fill.Reviewed honoured")

	issues := core.MarkReviewed(spec, doc, []core.Path{name, "", "nope.x", "personnes.pp[pp-1].etatCivil.dateNaissance"})
	assert.True(t, doc.Values[name].Reviewed)
	require.Len(t, issues, 3)
	assert.Equal(t, core.CodeUnknownPath, issues[0].Code)
	assert.Equal(t, core.CodeUnknownPath, issues[1].Code)
	assert.Equal(t, core.CodeNotFound, issues[2].Code)
}
