package core_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	pp1Name = "personnes.pp[pp-1].etatCivil.nomNaissance"
	prenoms = "personnes.pp[pp-1].etatCivil.prenom"
	total   = "operations.montantTotal"
)

// loadTracfinSpec returns the real TRACFIN spec of the current version.
func loadTracfinSpec(t *testing.T) *core.Spec {
	t.Helper()
	spec, ok := tracfin.Spec(tracfin.CurrentVersion)
	require.True(t, ok, "TRACFIN spec %s must load", tracfin.CurrentVersion)
	return spec
}

// docWithPerson returns a document holding one natural person, personnes.pp[pp-1].
func docWithPerson(t *testing.T) (*core.Document, core.Path) {
	t.Helper()
	doc := core.NewDocument()
	pp := doc.AddItem("personnes.pp", "pp")
	require.Equal(t, core.Path("personnes.pp[pp-1]"), pp)
	return doc, pp
}

func mustParsePath(t *testing.T, s string) core.Path {
	t.Helper()
	p, err := core.ParsePath(s)
	require.NoError(t, err)
	return p
}

func llmSource(object string, raw string) core.SourceRef {
	return core.SourceRef{
		Object:   core.ObjectRef{Table: "persons", ObjectID: object},
		Field:    "value",
		RawValue: raw,
	}
}

// llmCopiedFill builds a copied field written by the LLM, with one source.
func llmCopiedFill(path string, value string) core.Fill {
	return core.Fill{
		Path:    core.Path(path),
		Value:   value,
		Origin:  core.OriginLLM,
		Sources: []core.SourceRef{llmSource("person-1", value)},
	}
}

func valueOf(doc *core.Document, p core.Path) string {
	return doc.Values[p].Value
}

func snapshot(t *testing.T, doc *core.Document) string {
	t.Helper()
	b, err := json.Marshal(doc)
	require.NoError(t, err)
	return string(b)
}

func requireDocUnchanged(t *testing.T, before string, doc *core.Document) {
	t.Helper()
	assert.Equal(t, before, snapshot(t, doc), "document must not change")
}

func rejectedCodes(res core.ApplyResult) []string {
	codes := make([]string, 0, len(res.Rejected))
	for _, r := range res.Rejected {
		codes = append(codes, r.Issue.Code)
	}
	return codes
}

func requireNoProvisional(t *testing.T, res core.ApplyResult, doc *core.Document) {
	t.Helper()
	for _, p := range res.Applied {
		assert.NotContains(t, string(p), "[+")
	}
	for _, p := range doc.ItemContainers() {
		assert.NotContains(t, string(p), "[+")
		for _, k := range doc.ItemKeys(p) {
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

func TestApplyFills_AppliesValidValue(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	path := "personnes.pp[pp-1].etatCivil.nomNaissance"

	res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(path, "Dupont")}, core.ApplyOptions{})

	assert.Empty(t, res.Rejected)
	assert.Empty(t, res.Conflicts)
	assert.Equal(t, []core.Path{mustParsePath(t, path)}, res.Applied)
	assert.Equal(t, "Dupont", valueOf(doc, mustParsePath(t, path)))
	assert.Equal(t, core.OriginLLM, doc.Values[mustParsePath(t, path)].Origin)
	assert.Len(t, doc.Values[mustParsePath(t, path)].Sources, 1)
}

func TestApplyFills_PathOutsideAllowedPrefix(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, pp := docWithPerson(t)
	before := snapshot(t, doc)

	fills := []core.Fill{
		// Outside the delegated part (personnes.pp[pp-1]): must be refused.
		{Path: "operations.montantTotal", Value: "1000", Origin: core.OriginLLM,
			Sources: []core.SourceRef{llmSource("tx-1", "1000")}},
	}
	opts := core.ApplyOptions{AllowedPrefixes: []core.Path{pp}}

	res := core.ApplyFills(spec, doc, fills, opts)

	require.Len(t, res.Rejected, 1)
	assert.Equal(t, "out_of_scope", res.Rejected[0].Issue.Code)
	assert.Empty(t, res.Applied)
	requireDocUnchanged(t, before, doc)
}

func TestApplyFills_ManualValueBecomesConflict(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	path := mustParsePath(t, "personnes.pp[pp-1].etatCivil.nomNaissance")
	doc.Values[path] = core.Value{Value: "Dupont", Origin: core.OriginManual}
	before := snapshot(t, doc)

	res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(string(path), "Martin")},
		core.ApplyOptions{Overwrite: core.OverwriteNonManual})

	require.Len(t, res.Conflicts, 1)
	assert.Equal(t, path, res.Conflicts[0].Path)
	assert.Equal(t, "Dupont", res.Conflicts[0].Before.Value)
	assert.Equal(t, core.OriginManual, res.Conflicts[0].Before.Origin)
	assert.Equal(t, "Martin", res.Conflicts[0].After.Value)
	assert.Empty(t, res.Applied)
	requireDocUnchanged(t, before, doc)
}

func TestApplyFills_UnknownItemRejected(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t) // only pp-1 exists
	before := snapshot(t, doc)

	res := core.ApplyFills(spec, doc,
		[]core.Fill{llmCopiedFill("personnes.pp[pp-7].etatCivil.nomNaissance", "Dupont")},
		core.ApplyOptions{})

	require.Len(t, res.Rejected, 1)
	assert.Equal(t, core.CodeUnknownItem, res.Rejected[0].Issue.Code)
	assert.Empty(t, res.Applied)
	requireDocUnchanged(t, before, doc)
}

func TestApplyFills_InvalidValueRejected(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	before := snapshot(t, doc)

	// 2000-02-30 does not exist: the date field stores AAAA-MM-JJ.
	res := core.ApplyFills(spec, doc,
		[]core.Fill{llmCopiedFill("personnes.pp[pp-1].etatCivil.dateNaissance", "2000-02-30")},
		core.ApplyOptions{})

	require.Len(t, res.Rejected, 1)
	assert.Equal(t, core.CodeBadDate, res.Rejected[0].Issue.Code)
	assert.Empty(t, res.Applied)
	requireDocUnchanged(t, before, doc)
}

func TestApplyFills_EmptyValueClears(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	path := mustParsePath(t, "personnes.pp[pp-1].etatCivil.nomNaissance")
	doc.Values[path] = core.Value{Value: "Dupont", Origin: core.OriginLLM,
		Sources: []core.SourceRef{llmSource("person-1", "Dupont")}}

	res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(string(path), "")},
		core.ApplyOptions{Overwrite: core.OverwriteNonManual})

	assert.Empty(t, res.Rejected)
	assert.Empty(t, res.Conflicts)
	assert.Equal(t, []core.Path{path}, res.Applied)
	assert.Equal(t, "", valueOf(doc, path))
}

func TestApplyFills_ProvisionalKeyCreatesNestedItem(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)

	fills := []core.Fill{
		llmCopiedFill("personnes.pp[pp-1].etatCivil.prenom[+a]", "Jean"),
		llmCopiedFill("personnes.pp[pp-1].etatCivil.prenom[+b]", "Paul"),
	}

	res := core.ApplyFills(spec, doc, fills, core.ApplyOptions{})

	assert.Empty(t, res.Rejected)
	assert.Equal(t, []core.Path{
		mustParsePath(t, "personnes.pp[pp-1].etatCivil.prenom[prenom-1]"),
		mustParsePath(t, "personnes.pp[pp-1].etatCivil.prenom[prenom-2]"),
	}, res.Applied)
	assert.Equal(t, []string{"prenom-1", "prenom-2"}, doc.ItemKeys("personnes.pp[pp-1].etatCivil.prenom"))
	assert.Equal(t, "Jean", valueOf(doc, mustParsePath(t, "personnes.pp[pp-1].etatCivil.prenom[prenom-1]")))
	assert.Equal(t, "Paul", valueOf(doc, mustParsePath(t, "personnes.pp[pp-1].etatCivil.prenom[prenom-2]")))
}

func TestApplyFills_SameProvisionalKeySameItem(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)

	// Two fields of the same new identity (autreIdentite[+a]) must share one item.
	fills := []core.Fill{
		llmCopiedFill("personnes.pp[pp-1].etatCivil.autreIdentite[+a].nomNaissance", "Dupont"),
		llmCopiedFill("personnes.pp[pp-1].etatCivil.autreIdentite[+a].lieuNaissance", "Paris"),
	}

	res := core.ApplyFills(spec, doc, fills, core.ApplyOptions{})

	assert.Empty(t, res.Rejected)
	assert.Equal(t, []string{"autreIdentite-1"}, doc.ItemKeys("personnes.pp[pp-1].etatCivil.autreIdentite"))
	assert.Equal(t, []core.Path{
		mustParsePath(t, "personnes.pp[pp-1].etatCivil.autreIdentite[autreIdentite-1].nomNaissance"),
		mustParsePath(t, "personnes.pp[pp-1].etatCivil.autreIdentite[autreIdentite-1].lieuNaissance"),
	}, res.Applied)
}

func TestApplyFills_CopiedWithoutSourceRejected(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	before := snapshot(t, doc)

	fill := core.Fill{
		Path:   "personnes.pp[pp-1].etatCivil.nomNaissance",
		Value:  "Dupont",
		Origin: core.OriginLLM,
		// No source: a copied value must cite where it comes from.
	}

	res := core.ApplyFills(spec, doc, []core.Fill{fill}, core.ApplyOptions{})

	require.Len(t, res.Rejected, 1)
	assert.Equal(t, "missing_source", res.Rejected[0].Issue.Code)
	assert.Empty(t, res.Applied)
	requireDocUnchanged(t, before, doc)
}

func TestApplyFills_JudgmentWithoutJustificationRejected(t *testing.T) {
	spec := loadTracfinSpec(t)
	path := "personnes.pp[pp-1].implication" // judgment node, code list t_implication

	t.Run("no justification", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		before := snapshot(t, doc)
		fill := core.Fill{
			Path:       core.Path(path),
			Value:      "Victime présumée",
			Origin:     core.OriginLLM,
			Confidence: core.ConfidenceHigh,
		}

		res := core.ApplyFills(spec, doc, []core.Fill{fill}, core.ApplyOptions{})

		require.Len(t, res.Rejected, 1)
		assert.Equal(t, "missing_justification", res.Rejected[0].Issue.Code)
		assert.Empty(t, res.Applied)
		requireDocUnchanged(t, before, doc)
	})

	t.Run("no confidence", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		before := snapshot(t, doc)
		fill := core.Fill{
			Path:          core.Path(path),
			Value:         "Victime présumée",
			Origin:        core.OriginLLM,
			Justification: "Le signalement désigne la personne comme victime.",
		}

		res := core.ApplyFills(spec, doc, []core.Fill{fill}, core.ApplyOptions{})

		require.Len(t, res.Rejected, 1)
		assert.Equal(t, "missing_justification", res.Rejected[0].Issue.Code)
		assert.Empty(t, res.Applied)
		requireDocUnchanged(t, before, doc)
	})

	t.Run("justified and confident judgment is applied", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		fill := core.Fill{
			Path:          core.Path(path),
			Value:         "Victime présumée",
			Origin:        core.OriginLLM,
			Justification: "Le signalement désigne la personne comme victime.",
			Confidence:    core.ConfidenceHigh,
		}

		res := core.ApplyFills(spec, doc, []core.Fill{fill}, core.ApplyOptions{})

		assert.Empty(t, res.Rejected)
		assert.Equal(t, []core.Path{mustParsePath(t, path)}, res.Applied)
		assert.Equal(t, "Victime présumée", valueOf(doc, mustParsePath(t, path)))
	})
}

func TestApplyFills_LLMCannotWriteSettingOrComputed(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	before := snapshot(t, doc)

	fills := []core.Fill{
		// setting (declarant) and computed (operations, context) nodes
		{Path: "declarant.coordonnees.nom", Value: "Ada", Origin: core.OriginLLM,
			Sources: []core.SourceRef{llmSource("person-1", "Ada")}},
		{Path: "operations.montantTotal", Value: "1000", Origin: core.OriginLLM,
			Sources: []core.SourceRef{llmSource("tx-1", "1000")}},
		{Path: "context.general.refInterne", Value: "REF-1", Origin: core.OriginLLM,
			Sources: []core.SourceRef{llmSource("case-1", "REF-1")}},
	}

	res := core.ApplyFills(spec, doc, fills, core.ApplyOptions{})

	assert.Equal(t, []string{"not_writable_by_llm", "not_writable_by_llm", "not_writable_by_llm"},
		rejectedCodes(res))
	assert.Empty(t, res.Applied)
	requireDocUnchanged(t, before, doc)
}

func TestApplyFills_SourceOutOfScopeRejected(t *testing.T) {
	spec := loadTracfinSpec(t)

	allowed := func(target core.Path) []core.ObjectRef {
		return []core.ObjectRef{{Table: "persons", ObjectID: "person-1"}}
	}
	opts := core.ApplyOptions{AllowedSources: allowed}

	t.Run("source of another person is refused", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		before := snapshot(t, doc)
		fill := core.Fill{
			Path:    "personnes.pp[pp-1].etatCivil.nomNaissance",
			Value:   "Dupont",
			Origin:  core.OriginLLM,
			Sources: []core.SourceRef{{Object: core.ObjectRef{Table: "persons", ObjectID: "person-2"}, Field: "name", RawValue: "Dupont"}},
		}

		res := core.ApplyFills(spec, doc, []core.Fill{fill}, opts)

		require.Len(t, res.Rejected, 1)
		assert.Equal(t, "source_out_of_scope", res.Rejected[0].Issue.Code)
		assert.Empty(t, res.Applied)
		requireDocUnchanged(t, before, doc)
	})

	t.Run("source within scope is applied", func(t *testing.T) {
		doc, _ := docWithPerson(t)
		fill := core.Fill{
			Path:    "personnes.pp[pp-1].etatCivil.nomNaissance",
			Value:   "Dupont",
			Origin:  core.OriginLLM,
			Sources: []core.SourceRef{{Object: core.ObjectRef{Table: "persons", ObjectID: "person-1"}, Field: "name", RawValue: "Dupont"}},
		}

		res := core.ApplyFills(spec, doc, []core.Fill{fill}, opts)

		assert.Empty(t, res.Rejected)
		assert.Equal(t, "Dupont", valueOf(doc, mustParsePath(t, "personnes.pp[pp-1].etatCivil.nomNaissance")))
	})
}

func TestApplyFills_ReviewedValueProtected(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	path := mustParsePath(t, "personnes.pp[pp-1].etatCivil.nomNaissance")
	doc.Values[path] = core.Value{
		Value:    "Dupont",
		Origin:   core.OriginLLM,
		Sources:  []core.SourceRef{llmSource("person-1", "Dupont")},
		Reviewed: true,
	}
	before := snapshot(t, doc)

	res := core.ApplyFills(spec, doc, []core.Fill{llmCopiedFill(string(path), "Martin")},
		core.ApplyOptions{Overwrite: core.OverwriteNonManual})

	require.Len(t, res.Conflicts, 1)
	assert.Equal(t, "Dupont", res.Conflicts[0].Before.Value)
	assert.Empty(t, res.Applied)
	requireDocUnchanged(t, before, doc)
}

// A provisional item whose only fill is rejected must not stay as an orphan.
func TestApplyFills_RejectedProvisionalItemIsRemoved(t *testing.T) {
	spec := loadTracfinSpec(t)
	doc, _ := docWithPerson(t)
	before := snapshot(t, doc)

	fill := core.Fill{
		Path:   "personnes.pp[pp-1].etatCivil.prenom[+a]",
		Value:  "Jean",
		Origin: core.OriginLLM, // copied without source: rejected
	}

	res := core.ApplyFills(spec, doc, []core.Fill{fill}, core.ApplyOptions{})

	require.Len(t, res.Rejected, 1)
	assert.Equal(t, "missing_source", res.Rejected[0].Issue.Code)
	assert.Empty(t, res.Applied)
	assert.Empty(t, doc.ItemKeys("personnes.pp[pp-1].etatCivil.prenom"), "no orphan item")
	requireDocUnchanged(t, before, doc)
}

// Warning-only ValidateValue results (code_list_unavailable) are NOT covered
// here: no field of the TRACFIN 1.4.0 spec uses an unavailable code list
// (liste_ref_type_equipement.csv is unused), so there is no real field to test with.

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

// An origin that is not one of the defined constants must be refused, so
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
			assert.Equal(t, core.CodeUnknownOrigin, res.Rejected[0].Issue.Code)
			requireDocUnchanged(t, before, doc)
		})
	}
}

// A defined origin written by its legitimate writer is
// still accepted on the same setting field.
func TestApplyFills_DefinedOriginAccepted(t *testing.T) {
	spec := loadTracfinSpec(t)
	path := core.Path("declarant.coordonnees.nom")
	doc := core.NewDocument()
	res := core.ApplyFills(spec, doc, []core.Fill{{Path: path, Value: "Ada", Origin: core.OriginOrgSetting}}, core.ApplyOptions{})
	assert.Empty(t, res.Rejected)
	assert.Equal(t, []core.Path{path}, res.Applied)
}
