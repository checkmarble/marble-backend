package core_test

import (
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const total = "operations.montantTotal"

func TestApplyFills_SkipMaxItems(t *testing.T) {
	spec := loadTracfinSpec(t)
	build := func() []core.Fill {
		var fills []core.Fill
		for i := range 31 { // personnes.pp is limited to 30
			fills = append(fills, llmCopiedFill(fmt.Sprintf("personnes.pp[+k%d].etatCivil.nomNaissance", i), "Dupont"))
		}
		return fills
	}
	t.Run("enforced by default", func(t *testing.T) {
		doc := core.NewDocument()
		res := core.ApplyFills(spec, doc, build(), core.ApplyOptions{})
		assert.Len(t, res.Applied, 30)
		assert.Equal(t, []string{core.CodeTooManyItems}, rejectedCodes(res))
		assert.Len(t, doc.Items["personnes.pp"], 30)
	})
	t.Run("skipped", func(t *testing.T) {
		doc := core.NewDocument()
		res := core.ApplyFills(spec, doc, build(), core.ApplyOptions{SkipMaxItems: true})
		assert.Len(t, res.Applied, 31)
		assert.Empty(t, res.Rejected)
		assert.Len(t, doc.Items["personnes.pp"], 31)
	})
	t.Run("cleared field item does not count", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		for i := range 5 { // prenom max is 5
			item := doc.AddItem(core.Path(prenoms), "prenom")
			if i > 0 {
				doc.Values[item] = core.Value{Value: "Jean", Origin: core.OriginManual}
			}
		}
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(prenoms+"[+n]", "Paul")}, core.ApplyOptions{})
		assert.Empty(t, res.Rejected)
		assert.Len(t, res.Applied, 1)
		res = core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(prenoms+"[+m]", "Luc")}, core.ApplyOptions{})
		assert.Equal(t, []string{core.CodeTooManyItems}, rejectedCodes(res))
	})
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
	t.Run("analyst value stays protected from computed", func(t *testing.T) {
		doc := core.NewDocument()
		doc.Values[path] = core.Value{Value: "100", Origin: core.OriginManual}
		res := core.ApplyFills(spec, doc, []core.Fill{computed("200")}, core.ApplyOptions{Overwrite: core.OverwriteNonManual})
		assert.Len(t, res.Conflicts, 1)
		assert.Equal(t, "100", doc.Values[path].Value)
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

func TestApplyFills_ResolvedAndRewrite(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc := core.NewDocument()
	res := core.ApplyFills(spec, doc, []core.Fill{
		llmCopiedFill("personnes.pp[+a].etatCivil.nomNaissance", "Dupont"),
		llmCopiedFill("personnes.pp[+a].etatCivil.prenom[+x]", "Jean"),
		llmCopiedFill("personnes.pp[+a].etatCivil.prenom[+x]", "Jean"), // same item
	}, core.ApplyOptions{})
	require.Empty(t, res.Rejected)
	assert.Equal(t, map[core.Path]core.Path{
		"personnes.pp[+a]":                      "personnes.pp[pp-1]",
		"personnes.pp[+a].etatCivil.prenom[+x]": "personnes.pp[pp-1].etatCivil.prenom[prenom-1]",
	}, res.Resolved)
	assert.Equal(t, core.Path("personnes.pp[pp-1].etatCivil.nomNaissance"), res.Rewrite("personnes.pp[+a].etatCivil.nomNaissance"))
	assert.Equal(t, core.Path("personnes.pp[pp-1].etatCivil.prenom[prenom-1]"), res.Rewrite("personnes.pp[+a].etatCivil.prenom[+x]"))
	assert.Equal(t, core.Path("personnes.pp[pp-1]"), res.Rewrite("personnes.pp[+a]"))
	assert.Equal(t, core.Path("personnes.pp[+b].x"), res.Rewrite("personnes.pp[+b].x"))
	assert.Equal(t, core.Path("operations.montantTotal"), res.Rewrite("operations.montantTotal"))
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

func TestApplyFills_MetaChecks(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	noOrigin := llmCopiedFill(pp1Name, "Dupont")
	noOrigin.Origin = ""
	badConf := llmCopiedFill(pp1Name, "Dupont")
	badConf.Confidence = "certain"
	okConf := llmCopiedFill(pp1Name, "Dupont")
	okConf.Confidence = core.ConfidenceHigh
	res := core.ApplyFills(spec, doc, []core.Fill{noOrigin, badConf, okConf}, core.ApplyOptions{})
	assert.Equal(t, []string{core.CodeMissingOrigin, core.CodeBadConfidence}, rejectedCodes(res))
	assert.Len(t, res.Applied, 1)
}

// S1: an origin that is not one of the defined constants must be refused, so
// that the nature rules (which only look at OriginLLM) cannot be bypassed by a
// free string. declarant.coordonnees.nom is a setting field: the LLM may not write it.
func TestApplyFills_UnknownOriginRejected(t *testing.T) {
	spec := loadTracfinSpec(t)
	path := core.Path("declarant.coordonnees.nom")
	for _, origin := range []core.Origin{"LLM ", "Llm", "foo"} {
		t.Run(string(origin), func(t *testing.T) {
			doc := core.NewDocument()
			before := snapshot(t, doc)
			fill := core.Fill{Path: path, Value: "Ada", Origin: origin, Reviewed: true}

			res := core.ApplyFills(spec, doc, []core.Fill{fill}, core.ApplyOptions{})

			assert.Empty(t, res.Applied)
			require.Len(t, res.Rejected, 1)
			assert.True(t, res.Rejected[0].Issue.Blocks())
			requireDocUnchanged(t, before, doc)
		})
	}
}

// Guard (passes today): a defined origin written by its legitimate writer is
// still accepted on the same setting field.
func TestApplyFills_DefinedOriginAccepted(t *testing.T) {
	spec := loadTracfinSpec(t)
	path := core.Path("declarant.coordonnees.nom")
	doc := core.NewDocument()
	res := core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "Ada", Origin: core.OriginOrgSetting}}, core.ApplyOptions{})
	assert.Empty(t, res.Rejected)
	assert.Equal(t, []core.Path{path}, res.Applied)
}

func TestApplyFills_SameValueConfirmationOnlyAnalyst(t *testing.T) {
	spec := loadTracfinSpec(t)
	path := mustParsePath(t, pp1Name)
	for _, o := range []core.Origin{core.OriginLLM, core.OriginComputed} {
		doc, _ := docWithPerson(t)
		doc.Values[path] = core.Value{Value: "Dupont", Origin: core.OriginManual}
		f := llmCopiedFill(string(path), "Dupont")
		f.Origin = o
		res := core.ApplyFills(spec, doc, []core.Fill{f}, core.ApplyOptions{Overwrite: core.OverwriteAll})
		assert.Empty(t, res.Applied, string(o))
		assert.Equal(t, core.OriginManual, doc.Values[path].Origin)
	}
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

func TestApplyFills_EmptyNewItemAndBadProvisionalKey(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	before := snapshot(t, doc)
	res := core.ApplyFills(spec, doc, []core.Fill{
		{Path: core.Path(prenoms + "[+a]"), Value: "", Origin: core.OriginManual},
		llmCopiedFill("personnes.pp[pp-1].etatCivil[+a].nomNaissance", "Dupont"),
	}, core.ApplyOptions{})
	codes := rejectedCodes(res)
	require.Len(t, codes, 2)
	assert.Equal(t, core.CodeEmptyNewItem, codes[0])
	assert.Contains(t, []string{core.CodeBadProvisionalKey, core.CodeUnknownPath}, codes[1])
	requireDocUnchanged(t, before, doc)
}

func TestApplyFills_AllowedSourcesSeesProvisionalKeyOfNewItem(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc := core.NewDocument()
	var targets []core.Path
	opts := core.ApplyOptions{AllowedSources: func(target core.Path) []core.ObjectRef {
		targets = append(targets, target)
		return []core.ObjectRef{{Table: "persons", ObjectID: "person-1"}}
	}}
	res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill("personnes.pp[+a].etatCivil.nomNaissance", "Dupont")}, opts)
	require.Empty(t, res.Rejected)
	assert.Equal(t, []core.Path{"personnes.pp[+a].etatCivil.nomNaissance"}, targets)
}
