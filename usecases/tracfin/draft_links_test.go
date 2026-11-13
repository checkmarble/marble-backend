package tracfin

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

func TestTracfinDraftRemovePersonDetachesReferenceWithoutLosingCorrection(t *testing.T) {
	c := draftCatalog(t)
	const field = "/ds/operations/operation/emetteurTypeEmetteur"
	text := "human correction"
	d := editDraft(models.TracfinBlock{ID: "root", CatalogID: "/ds", Children: []models.TracfinBlock{
		{ID: "people", CatalogID: "/ds/personnes", Children: []models.TracfinBlock{{ID: "person", CatalogID: "/ds/personnes/pp"}}},
		{ID: "ops", CatalogID: "/ds/operations", Children: []models.TracfinBlock{{ID: "op", CatalogID: "/ds/operations/operation", Fields: map[string][]models.TracfinValue{
			field:                                   {{ID: "ref", Kind: "reference", Reference: &models.TracfinReference{BlockID: "person"}}},
			"/ds/operations/operation/commentaires": {{ID: "correction", Kind: "text", Text: &text, Origin: &models.TracfinOrigin{Source: "manual"}}},
		}}}},
	}})
	got, err := ApplyChanges(d, []Change{{Action: "remove_block", BlockID: "person"}}, c)
	require.NoError(t, err)
	require.Empty(t, got.Document.Root.Children[0].Children)
	op := got.Document.Root.Children[1].Children[0]
	require.Equal(t, "ref", op.Fields[field][0].ID)
	require.Equal(t, "person", op.Fields[field][0].Reference.BlockID)
	require.True(t, op.Fields[field][0].NeedsReview)
	require.Equal(t, "human correction", *op.Fields["/ds/operations/operation/commentaires"][0].Text)
	require.Equal(t, "correction", op.Fields["/ds/operations/operation/commentaires"][0].ID)
}

func TestTracfinDraftDistinctDecisionsRemainSeparate(t *testing.T) {
	c := draftCatalog(t)
	d := editDraft(models.TracfinBlock{ID: "ops", CatalogID: "/ds/operations"})
	changes := []Change{
		{Action: "add_block", ParentBlockID: "ops", BlockID: "a", CatalogID: "/ds/operations/operation"},
		{Action: "set_selection", BlockID: "a", Selection: &models.TracfinSelection{MappingID: "m", DecisionID: "d1", HistoricalType: "Payment"}},
		{Action: "add_block", ParentBlockID: "ops", BlockID: "b", CatalogID: "/ds/operations/operation"},
		{Action: "set_selection", BlockID: "b", Selection: &models.TracfinSelection{MappingID: "m", DecisionID: "d2", HistoricalType: "Payment"}},
	}
	got, err := ApplyChanges(d, changes, c)
	require.NoError(t, err)
	require.Len(t, got.Document.Root.Children, 2)
	require.Equal(t, "d1", got.Document.Root.Children[0].Selection.DecisionID)
	require.Equal(t, "d2", got.Document.Root.Children[1].Selection.DecisionID)
}
