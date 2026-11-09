package tracfin

import (
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

func scopeDraft(c Catalog) models.TracfinDraft {
	return task5Draft(c, task5Block("people", "/ds/personnes", task5Block("pm", "/ds/personnes/pm")))
}

func scopeValue(key, block, field, text string) AIValue {
	return AIValue{Key: key, BlockKey: block, FieldID: field, Kind: "text", Text: &text, Explanation: "from source", Sources: []AIEvidence{{SourceID: "s", Locator: "/x"}}}
}

func scopeErrors(ds []models.TracfinDiagnostic) []models.TracfinDiagnostic {
	var out []models.TracfinDiagnostic
	for _, d := range ds {
		if d.Severity == "error" {
			out = append(out, d)
		}
	}
	return out
}

func TestAIValueOnBlockExplainsChildFields(t *testing.T) {
	c := task5Catalog(t)
	p := AISectionProposal{
		Blocks: []AIBlock{{Key: "activity", ParentKey: "pm", CatalogID: "/ds/personnes/pm/activiteCoordonnee"}},
		Values: []AIValue{scopeValue("v", "activity", "/ds/personnes/pm/activiteCoordonnee", "Je ne sais pas")},
	}
	_, ds := ApplyAISection(scopeDraft(c), "persons", p, map[string]bool{"s": true}, c, "run")
	errs := scopeErrors(ds)
	require.Len(t, errs, 1, "%+v", ds)
	require.Equal(t, "FIELD_IS_BLOCK", errs[0].Code)
	require.Contains(t, errs[0].Message, "/ds/personnes/pm/activiteCoordonnee/numeroIdentification/@type")
}

func TestAICalculatedValueIsIgnored(t *testing.T) {
	c := task5Catalog(t)
	require.Equal(t, "calculated", c.Fields["/ds/personnes/pm/id"].Coverage)
	p := AISectionProposal{Values: []AIValue{scopeValue("id", "pm", "/ds/personnes/pm/id", "2")}}
	out, ds := ApplyAISection(scopeDraft(c), "persons", p, map[string]bool{"s": true}, c, "run")
	require.Empty(t, scopeErrors(ds), "%+v", ds)
	require.Empty(t, out.Document.Root.Children[0].Children[0].Fields["/ds/personnes/pm/id"])
}

func TestAIValueErrorsAreReportedTogether(t *testing.T) {
	c := task5Catalog(t)
	p := AISectionProposal{Values: []AIValue{
		scopeValue("a", "pm", "/ds/personnes/pm/activiteCoordonnee", "x"),
		scopeValue("b", "pm", "/ds/context/general/refInterne", "x"),
		scopeValue("c", "missing-block", "/ds/personnes/pm/activiteCoordonnee/denominationSociale", "x"),
		scopeValue("d", "pm", "/ds/personnes/pm/activiteCoordonnee/denominationSociale", "x"),
	}}
	_, ds := ApplyAISection(scopeDraft(c), "persons", p, map[string]bool{"s": true}, c, "run")
	codes := []string{}
	for _, d := range scopeErrors(ds) {
		codes = append(codes, d.Code)
	}
	require.Equal(t, []string{"FIELD_IS_BLOCK", "SECTION_SCOPE", "VALUE_BLOCK_UNKNOWN", "FIELD_BLOCK_MISMATCH"}, codes, "%+v", ds)
	for _, d := range scopeErrors(ds) {
		require.True(t, strings.HasPrefix(d.Message, "value "), d.Message)
	}
	require.Contains(t, scopeErrors(ds)[1].Message, "declaration")
	require.Contains(t, scopeErrors(ds)[3].Message, "/ds/personnes/pm/activiteCoordonnee")
}

func TestAIBlockScopeNamesTheProblem(t *testing.T) {
	c := task5Catalog(t)
	for _, tc := range []struct {
		catalogID, want string
	}{
		{"/ds/personnes/pm/activiteCoordonnee/denominationSociale", "is a field"},
		{"/ds/context/general", "declaration"},
		{"/ds/personnes/unknown", "not in the catalog"},
	} {
		p := AISectionProposal{Blocks: []AIBlock{{Key: "k", ParentKey: "pm", CatalogID: tc.catalogID}}}
		_, ds := ApplyAISection(scopeDraft(c), "persons", p, map[string]bool{"s": true}, c, "run")
		errs := scopeErrors(ds)
		require.Len(t, errs, 1, "%+v", ds)
		require.Equal(t, "SECTION_SCOPE", errs[0].Code)
		require.Contains(t, errs[0].Message, tc.want)
	}
}

func managerDraft(c Catalog) models.TracfinDraft {
	return task5Draft(c, task5Block("people", "/ds/personnes",
		task5Block("pp", "/ds/personnes/pp"),
		task5Block("pm", "/ds/personnes/pm", task5Block("manager", "/ds/personnes/pm/dirigeant"))))
}

func managerRef(key, target string) AIValue {
	return AIValue{Key: key, BlockKey: "manager", FieldID: "/ds/personnes/pm/dirigeant/idPersonne", Kind: "reference", ReferenceKey: target, Explanation: "gérante", Sources: []AIEvidence{{SourceID: "s", Locator: "/x"}}}
}

func TestAIIdenticalDuplicateValueIsIdempotent(t *testing.T) {
	c := task5Catalog(t)
	p := AISectionProposal{Values: []AIValue{managerRef("batch", "pp"), managerRef("final", "pp")}}
	out, ds := ApplyAISection(managerDraft(c), "persons", p, map[string]bool{"s": true}, c, "run")
	for _, d := range ds {
		require.NotEqual(t, "CARDINALITY_EXCEEDED", d.Code, "%+v", ds)
		require.NotEqual(t, "FIELD_CONTRACT", d.Code, "%+v", ds)
	}
	manager := out.Document.Root.Children[0].Children[1].Children[0]
	require.Len(t, manager.Fields["/ds/personnes/pm/dirigeant/idPersonne"], 1)
}

func TestAIReferenceIgnoresExtraneousMembers(t *testing.T) {
	c := task5Catalog(t)
	v := managerRef("ref", "pp")
	one := "1"
	v.Text = &one
	_, ds := ApplyAISection(managerDraft(c), "persons", AISectionProposal{Values: []AIValue{v}}, map[string]bool{"s": true}, c, "run")
	require.Empty(t, scopeErrors(ds), "%+v", ds)
}

func TestAIReferenceWithoutReferenceKeyExplainsFix(t *testing.T) {
	c := task5Catalog(t)
	v := managerRef("ref", "")
	v.PersonReferenceKey = "pp"
	_, ds := ApplyAISection(managerDraft(c), "persons", AISectionProposal{Values: []AIValue{v}}, map[string]bool{"s": true}, c, "run")
	errs := scopeErrors(ds)
	require.Len(t, errs, 1)
	require.Contains(t, errs[0].Message, "reference_key is required")
	require.Contains(t, errs[0].Message, "person_reference_key")
}
