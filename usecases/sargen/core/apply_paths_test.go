package core_test

import (
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ApplyFills paths: provisional keys, allowed prefixes, item references and
// the maximum item count of repeatable containers.

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
	assert.Equal(t, []string{"pp-1"}, doc.ItemKeys("personnes.pp"))
	assert.Equal(t, []string{"prenom-1", "prenom-2"}, doc.ItemKeys(core.Path(prenoms)))
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
	assert.Len(t, doc.ItemKeys(core.Path(prenoms)), 5)
}

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
		assert.Len(t, doc.ItemKeys("personnes.pp"), 30)
	})
	t.Run("skipped", func(t *testing.T) {
		doc := core.NewDocument()
		res := core.ApplyFills(spec, doc, build(), core.ApplyOptions{SkipMaxItems: true})
		assert.Len(t, res.Applied, 31)
		assert.Empty(t, res.Rejected)
		assert.Len(t, doc.ItemKeys("personnes.pp"), 31)
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

func TestPathAllowed(t *testing.T) {
	pre := []core.Path{"personnes.pp[pp-1]", "operations.montantTotal", "organisme"}
	assert.True(t, core.PathAllowed(pp1Name, pre))
	assert.True(t, core.PathAllowed("organisme.libelle", pre))
	assert.True(t, core.PathAllowed("operations.montantTotal", pre))
	assert.False(t, core.PathAllowed("personnes.pp[pp-2].etatCivil.nomNaissance", pre))
	assert.False(t, core.PathAllowed("organismeX.libelle", pre))
	assert.True(t, core.PathAllowed("personnes.pp[pp-1]", []core.Path{"personnes.pp"}), "a container prefix allows its items")
	assert.False(t, core.PathAllowed(pp1Name, nil), "no prefix allows nothing")
	assert.False(t, core.PathAllowed(pp1Name, []core.Path{""}))
}
