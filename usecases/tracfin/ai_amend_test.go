package tracfin

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

const amendName = "/ds/personnes/pp/etatCivil/nomDusage"

func amendDraft(c Catalog) models.TracfinDraft {
	ai := func(source string) *models.TracfinOrigin {
		return &models.TracfinOrigin{Source: source, Resolution: "resolved", RunID: "parent"}
	}
	old := "Petit"
	civil := task5Block("civil", "/ds/personnes/pp/etatCivil")
	civil.Origin = ai("ai")
	civil.Fields[amendName] = []models.TracfinValue{{ID: "old-name", Kind: "text", Text: &old, Origin: ai("ai")}}
	alias := "Lou"
	civil.Fields["/ds/personnes/pp/etatCivil/alias"] = []models.TracfinValue{{ID: "old-alias", Kind: "text", Text: &alias, Origin: ai("ai")}}
	first := "Louis"
	civil.Fields["/ds/personnes/pp/etatCivil/prenom"] = []models.TracfinValue{{ID: "manual-first", Kind: "text", Text: &first, Origin: ai("manual")}}
	pp := task5Block("pp", "/ds/personnes/pp", civil)
	pp.Origin = ai("ai")
	other := task5Block("pp2", "/ds/personnes/pp")
	other.Origin = ai("ai")
	return task5Draft(c, task5Block("people", "/ds/personnes", pp, other))
}

func amendValue(block, field, text string) AIValue {
	return AIValue{Key: "v-" + field, BlockKey: block, FieldID: field, Kind: "text", Text: &text, Explanation: "remarque de l'analyste", Sources: []AIEvidence{{SourceID: "s", Locator: "/x"}}}
}

func amendFind(d models.TracfinDraft, id string) *models.TracfinBlock {
	return findBlock(&d.Document.Root, id)
}

func TestApplyAIAmendReplacesInPlaceAndKeepsIDs(t *testing.T) {
	c := task5Catalog(t)
	p := AIAmendProposal{AISectionProposal{Values: []AIValue{amendValue("civil", amendName, "PETIT")}}, nil, nil}
	out, ds := ApplyAIAmend(amendDraft(c), "persons", p, map[string]bool{"s": true}, c, "child")
	require.Empty(t, scopeErrorsOf(ds), "%+v", ds)
	civil := amendFind(out, "civil")
	require.NotNil(t, civil, "the block keeps its ID, so references to it stay valid")
	require.Len(t, civil.Fields[amendName], 1, "a generated value is replaced, not duplicated")
	require.Equal(t, "PETIT", *civil.Fields[amendName][0].Text)
	require.Equal(t, "child", civil.Fields[amendName][0].Origin.RunID)
	require.NotNil(t, amendFind(out, "pp2"), "untouched blocks stay")
}

func TestApplyAIAmendProtectsManualValues(t *testing.T) {
	c := task5Catalog(t)
	p := AIAmendProposal{AISectionProposal{Values: []AIValue{amendValue("civil", "/ds/personnes/pp/etatCivil/prenom", "Luis")}}, nil, nil}
	_, ds := ApplyAIAmend(amendDraft(c), "persons", p, map[string]bool{"s": true}, c, "child")
	require.NotEmpty(t, scopeErrorsOf(ds))
	require.Equal(t, "MANUAL_PROTECTED", scopeErrorsOf(ds)[0].Code)
}

func TestApplyAIAmendRemovesGeneratedBlocksAndValues(t *testing.T) {
	c := task5Catalog(t)
	p := AIAmendProposal{RemoveBlocks: []string{"pp2"}, ClearValues: []AIValueRef{{BlockKey: "civil", FieldID: "/ds/personnes/pp/etatCivil/alias"}}}
	out, ds := ApplyAIAmend(amendDraft(c), "persons", p, map[string]bool{"s": true}, c, "child")
	require.Empty(t, scopeErrorsOf(ds), "%+v", ds)
	require.Nil(t, amendFind(out, "pp2"))
	require.Empty(t, amendFind(out, "civil").Fields["/ds/personnes/pp/etatCivil/alias"])
	require.Len(t, amendFind(out, "civil").Fields[amendName], 1, "other values stay")
}

func TestApplyAIAmendRefusesOutOfScopeRemovals(t *testing.T) {
	c := task5Catalog(t)
	for _, tc := range []struct {
		name string
		p    AIAmendProposal
		code string
	}{
		{"unknown block", AIAmendProposal{RemoveBlocks: []string{"missing"}}, "AMEND_TARGET_UNKNOWN"},
		{"other section", AIAmendProposal{RemoveBlocks: []string{"pp"}}, "AMEND_SCOPE"},
		{"manual value", AIAmendProposal{ClearValues: []AIValueRef{{BlockKey: "civil", FieldID: "/ds/personnes/pp/etatCivil/prenom"}}}, "MANUAL_PROTECTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			section := "persons"
			if tc.name == "other section" {
				section = "operations"
			}
			_, ds := ApplyAIAmend(amendDraft(c), section, tc.p, map[string]bool{"s": true}, c, "child")
			require.NotEmpty(t, scopeErrorsOf(ds))
			require.Equal(t, tc.code, scopeErrorsOf(ds)[0].Code)
		})
	}
}

func scopeErrorsOf(ds []models.TracfinDiagnostic) []models.TracfinDiagnostic {
	var out []models.TracfinDiagnostic
	for _, d := range ds {
		if d.Severity == "error" {
			out = append(out, d)
		}
	}
	return out
}

func TestSetFieldReplacesASingleDerivedValueButNotAManualOne(t *testing.T) {
	c := task5Catalog(t)
	d := amendDraft(c)
	text := "PETIT"
	out, err := ApplyChanges(d, []Change{{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "civil", FieldID: amendName}, Value: &models.TracfinFieldInput{Kind: "text", Text: &text}}}, c)
	require.NoError(t, err)
	values := findBlock(&out.Document.Root, "civil").Fields[amendName]
	require.Len(t, values, 1)
	require.Equal(t, "manual", values[0].Origin.Source)
	_, err = ApplyChanges(out, []Change{{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "civil", FieldID: amendName}, Value: &models.TracfinFieldInput{Kind: "text", Text: &text}}}, c)
	require.Error(t, err, "a manual value is replaced only when its occurrence is named")
}
