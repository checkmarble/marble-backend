package tracfin

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTracfinDraftRemoveReferencedPersonOrSupport(t *testing.T) {
	for _, tc := range []struct {
		name, target, field string
		ref                 models.TracfinReference
	}{
		{"person", "person", "/ds/operations/operation/emetteurTypeEmetteur", models.TracfinReference{BlockID: "person"}},
		{"support", "support", "/ds/operations/operation/refFinanciereEmission", models.TracfinReference{BlockID: "support", PersonBlockID: "person"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := draftCatalog(t)
			other := "keep correction"
			d := editDraft(models.TracfinBlock{ID: "root", CatalogID: "/ds", Children: []models.TracfinBlock{
				{ID: "people", CatalogID: "/ds/personnes", Children: []models.TracfinBlock{{ID: "person", CatalogID: "/ds/personnes/pp", Children: []models.TracfinBlock{{ID: "refs", CatalogID: "/ds/personnes/pp/referencesFinancieres", Children: []models.TracfinBlock{{ID: "support", CatalogID: "/ds/personnes/pp/referencesFinancieres/referenceFinanciereIban"}, {ID: "other-support", CatalogID: "/ds/personnes/pp/referencesFinancieres/referenceFinanciereIban"}}}}}, {ID: "other-person", CatalogID: "/ds/personnes/pp"}}},
				{ID: "ops", CatalogID: "/ds/operations", Children: []models.TracfinBlock{{ID: "op", CatalogID: "/ds/operations/operation", Fields: map[string][]models.TracfinValue{tc.field: {{ID: "reference", Kind: "reference", Reference: &tc.ref}}, "/ds/operations/operation/commentaires": {{ID: "manual", Kind: "text", Text: &other, Origin: &models.TracfinOrigin{Source: "manual"}}}}}}},
			}})
			got, err := ApplyChanges(d, []Change{{Action: "remove_block", BlockID: tc.target}}, c)
			require.NoError(t, err)
			fields := got.Document.Root.Children[1].Children[0].Fields
			require.Equal(t, tc.ref, *fields[tc.field][0].Reference)
			require.Equal(t, "reference", fields[tc.field][0].ID)
			require.True(t, fields[tc.field][0].NeedsReview)
			require.Equal(t, "keep correction", *fields["/ds/operations/operation/commentaires"][0].Text)
		})
	}
}

func TestTracfinDraftSelectionChangedParentMarksAffectedValueForReview(t *testing.T) {
	c := draftCatalog(t)
	selection := models.TracfinSelection{MappingID: "m", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U"}, Bindings: []models.TracfinPathStep{{ID: "account", ParentID: "root", Table: "Account", RecordID: "A2", Link: "owner", Direction: "inverse"}, {ID: "card", ParentID: "account", Table: "Card", RecordID: "C2", Link: "account", Direction: "inverse"}}}
	corrected := "corrected"
	const field = "/ds/declarant/coordonnees/nom"
	d := editDraft(models.TracfinBlock{ID: "p", CatalogID: "/ds/declarant/coordonnees", Selection: &selection, Fields: map[string][]models.TracfinValue{field: {{ID: "v", Kind: "text", Text: &corrected, Origin: &models.TracfinOrigin{Source: "ingested", Resolution: "resolved", Sources: []models.TracfinSourceRef{{Kind: "ingested", RecordID: "C2", Path: selection.Bindings}}}}}}})
	changed := selection
	changed.Bindings = append([]models.TracfinPathStep(nil), selection.Bindings...)
	changed.Bindings[0].RecordID = "A3"
	got, err := ApplyChanges(d, []Change{{Action: "set_selection", BlockID: "p", Selection: &changed}}, c)
	require.NoError(t, err)
	require.Len(t, got.Document.Root.Selection.Bindings, 1)
	v := got.Document.Root.Fields[field][0]
	require.Equal(t, "v", v.ID)
	require.Equal(t, "corrected", *v.Text)
	require.True(t, v.NeedsReview)
	require.Equal(t, "needs_review", v.Origin.Resolution)
}
