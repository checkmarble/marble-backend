package core_test

import (
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	pp1Name = "personnes.pp[pp-1].etatCivil.nomNaissance"
	prenoms = "personnes.pp[pp-1].etatCivil.prenom"
)

func requireNoProvisional(t *testing.T, res core.ApplyResult, doc *core.Document) {
	t.Helper()
	for _, p := range res.Applied {
		assert.NotContains(t, string(p), "[+")
	}
	for p, keys := range doc.Items {
		assert.NotContains(t, string(p), "[+")
		for _, k := range keys {
			assert.False(t, strings.Contains(k, "+"))
		}
	}
	for p := range doc.NextKey {
		assert.NotContains(t, string(p), "[+")
	}
	for p, v := range doc.Values {
		assert.NotContains(t, string(p), "[+")
		assert.NotContains(t, v.Value, "[+")
	}
}

func TestApplyFills_NestedProvisionalAcrossFills(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc := core.NewDocument()
	fills := []core.Fill{
		llmCopiedFill("personnes.pp[+a].etatCivil.nomNaissance", "Dupont"),
		llmCopiedFill("personnes.pp[+a].etatCivil.prenom[+x]", "Jean"),
		llmCopiedFill("personnes.pp[+a].etatCivil.prenom[+y]", "Paul"),
		llmCopiedFill("personnes.pp[+a].etatCivil.prenom[+x]", "Jean-Pierre"),
	}
	res := core.ApplyFills(spec, doc, fills, core.ApplyOptions{})

	assert.Empty(t, res.Rejected)
	assert.Len(t, res.Conflicts, 1) // same +x written twice: default policy keeps the first value
	requireNoProvisional(t, res, doc)
	assert.Equal(t, []string{"pp-1"}, doc.Items["personnes.pp"])
	assert.Equal(t, []string{"prenom-1", "prenom-2"}, doc.Items[core.Path(prenoms)])
	assert.Equal(t, "Jean", valueOf(doc, mustParsePath(t, prenoms+"[prenom-1]")))
	assert.Equal(t, "Paul", valueOf(doc, mustParsePath(t, prenoms+"[prenom-2]")))
	assert.Equal(t, "Dupont", valueOf(doc, mustParsePath(t, pp1Name)))
}

func TestApplyFills_ContainerPrefixes(t *testing.T) {
	spec := loadTracfinSpec(t)
	tx := func(path string) core.Fill { return llmCopiedFill(path, "personnes.pp[pp-1]") }

	t.Run("container prefix allows new and existing items", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		opts := core.ApplyOptions{AllowedPrefixes: []core.Path{"work.transactions"}}
		res := core.ApplyFills(spec, doc, []core.Fill{tx("work.transactions[+x].emetteurPersonne")}, opts)
		require.Empty(t, res.Rejected)
		require.Len(t, res.Applied, 1)
		res = core.ApplyFills(spec, doc, []core.Fill{tx(string(res.Applied[0].Parent()) + ".beneficiairePersonne")}, opts)
		assert.Empty(t, res.Rejected)
		assert.Len(t, res.Applied, 1)
	})
	t.Run("nested container prefix allows new items", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(prenoms+"[+x]", "Jean")},
			core.ApplyOptions{AllowedPrefixes: []core.Path{prenoms}})
		assert.Empty(t, res.Rejected)
		assert.Len(t, res.Applied, 1)
	})
	t.Run("item prefix allows new nested items", func(t *testing.T) {
		doc, pp := docWithPerson(t)
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(prenoms+"[+x]", "Jean")},
			core.ApplyOptions{AllowedPrefixes: []core.Path{pp}})
		assert.Empty(t, res.Rejected)
	})
	t.Run("other container is out of scope", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		before := snapshot(t, doc)
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(prenoms+"[+x]", "Jean")},
			core.ApplyOptions{AllowedPrefixes: []core.Path{"work.transactions"}})
		assert.Equal(t, []string{core.CodeOutOfScope}, rejectedCodes(res))
		requireDocUnchanged(t, before, doc)
	})
	t.Run("empty prefix never matches everything", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(pp1Name, "Dupont")},
			core.ApplyOptions{AllowedPrefixes: []core.Path{""}})
		assert.Equal(t, []string{core.CodeOutOfScope}, rejectedCodes(res))
	})
}

func TestApplyFills_References(t *testing.T) {
	spec := loadTracfinSpec(t)
	const field = ".emetteurPersonne"

	t.Run("provisional ref created earlier is rewritten", func(t *testing.T) {
		doc := core.NewDocument()
		res := core.ApplyFills(spec, doc, []core.Fill{
			llmCopiedFill("personnes.pp[+z].etatCivil.nomNaissance", "Dupont"),
			llmCopiedFill("work.transactions[+t]"+field, "personnes.pp[+z]"),
		}, core.ApplyOptions{})
		assert.Empty(t, res.Rejected)
		requireNoProvisional(t, res, doc)
		assert.Equal(t, "personnes.pp[pp-1]", valueOf(doc, mustParsePath(t, "work.transactions[transactions-1]"+field)))
	})
	t.Run("provisional ref never created is rejected", func(t *testing.T) {
		doc := core.NewDocument()
		before := snapshot(t, doc)
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill("work.transactions[+t]"+field, "personnes.pp[+z]")}, core.ApplyOptions{})
		assert.Equal(t, []string{core.CodeBadRef}, rejectedCodes(res))
		requireDocUnchanged(t, before, doc)
	})
	t.Run("ref to missing real item is dangling", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		before := snapshot(t, doc)
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill("work.transactions[+t]"+field, "personnes.pp[pp-9]")}, core.ApplyOptions{})
		assert.Equal(t, []string{core.CodeDanglingRef}, rejectedCodes(res))
		requireDocUnchanged(t, before, doc)
	})
	t.Run("free text of a ref-or-text field is untouched", func(t *testing.T) {
		doc := core.NewDocument()
		res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill("work.transactions[+t].refEmission", "virement [+x] libre")}, core.ApplyOptions{})
		assert.Empty(t, res.Rejected)
		assert.Equal(t, "virement [+x] libre", valueOf(doc, res.Applied[0]))
	})
}

func TestApplyFills_AnalystConfirmationOfSameValue(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	path := mustParsePath(t, pp1Name)
	doc.Values[path] = core.Value{Value: "Dupont", Origin: core.OriginLLM, Sources: []core.SourceRef{llmSource("p", "Dupont")}}

	res := core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "Dupont", Origin: core.OriginManual}},
		core.ApplyOptions{Overwrite: core.OverwriteAll})
	assert.Equal(t, []core.Path{path}, res.Applied)
	assert.Equal(t, core.OriginManual, doc.Values[path].Origin)

	res = core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(string(path), "Martin")},
		core.ApplyOptions{Overwrite: core.OverwriteNonManual})
	require.Len(t, res.Conflicts, 1)
	assert.Equal(t, "Dupont", valueOf(doc, path))
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
	t.Run("non manual: analyst origin is protected", func(t *testing.T) {
		doc := existing(core.OriginAnalyst)
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

func TestApplyFills_MaxItemsOnProvisionalCreation(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	var fills []core.Fill
	for _, k := range []string{"a", "b", "c", "d", "e", "f"} {
		fills = append(fills, llmCopiedFill(prenoms+"[+"+k+"]", "Jean"))
	}
	res := core.ApplyFills(spec, doc, fills, core.ApplyOptions{})
	assert.Len(t, res.Applied, 5)
	assert.Equal(t, []string{core.CodeTooManyItems}, rejectedCodes(res))
	assert.Len(t, doc.Items[core.Path(prenoms)], 5)
}
