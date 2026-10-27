package tracfin

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

func TestTracfinSelectionExplicitReplacementChildSurvives(t *testing.T) {
	c := draftCatalog(t)
	old := models.TracfinSelection{MappingID: "m", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U"}, Bindings: []models.TracfinPathStep{{ID: "account", ParentID: "root", Table: "Account", RecordID: "A2", Link: "owner", Direction: "inverse"}, {ID: "card", ParentID: "account", Table: "Card", RecordID: "C2", Link: "account", Direction: "inverse"}}}
	d := editDraft(models.TracfinBlock{ID: "p", CatalogID: "/ds/declarant/coordonnees", Selection: &old})
	replacement := old
	replacement.Bindings = append([]models.TracfinPathStep(nil), old.Bindings...)
	replacement.Bindings[0].RecordID = "A3"
	replacement.Bindings[1].RecordID = "C3"
	got, err := ApplyChanges(d, []Change{{Action: "set_selection", BlockID: "p", Selection: &replacement}}, c)
	require.NoError(t, err)
	require.Equal(t, replacement, *got.Document.Root.Selection)
	require.Equal(t, "A2", d.Document.Root.Selection.Bindings[0].RecordID)
	require.Equal(t, "C2", d.Document.Root.Selection.Bindings[1].RecordID)
}
