package tracfin

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

// Change is the editing command; the API input contains no trusted origin.
func draftChange(action, block, field, valueID string, value *models.TracfinFieldInput) Change {
	return Change{Action: action, Field: models.TracfinFieldRef{BlockID: block, FieldID: field, ValueID: valueID}, Value: value}
}
func textInput(s string) *models.TracfinFieldInput {
	return &models.TracfinFieldInput{Kind: "text", Text: &s}
}
func draftCatalog(t *testing.T) Catalog {
	t.Helper()
	c, err := LoadCatalog()
	require.NoError(t, err)
	return c
}
func editDraft(block models.TracfinBlock) models.TracfinDraft {
	return models.TracfinDraft{Document: models.TracfinDocument{Root: block}}
}

func TestTracfinManualOverrideSurvivesPrefill(t *testing.T) {
	const field = "/ds/declarant/coordonnees/nom"
	c := draftCatalog(t)
	d := editDraft(models.TracfinBlock{ID: "p", CatalogID: "/ds/declarant/coordonnees"})
	cfg := models.TracfinConfiguration{Mappings: []models.TracfinMapping{{ID: "m", TargetField: field, Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "name"}}}}
	d.Document.Root.Selection = &models.TracfinSelection{MappingID: "m", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U"}}
	d.Configuration = cfg
	edited, err := ApplyChanges(d, []Change{draftChange("set_field", "p", field, "", textInput("manual"))}, c)
	require.NoError(t, err)
	values := edited.Document.Root.Fields[field]
	require.Len(t, values, 1)
	require.NotEmpty(t, values[0].ID)
	require.Equal(t, "manual", *values[0].Text)
	sources := SourceSet{Records: map[SourceKey]SourceRecord{{"p", "root"}: {Table: "User", RecordID: "U", Data: map[string]any{"name": "changed source"}}}}
	require.Empty(t, ResolvePrefill(cfg, edited, sources).Proposals)
	cleared, err := ApplyChanges(edited, []Change{draftChange("clear_field", "p", field, values[0].ID, nil)}, c)
	require.NoError(t, err)
	require.Equal(t, "cleared", cleared.Document.Root.Fields[field][0].Kind)
	require.Empty(t, ResolvePrefill(cfg, cleared, sources).Proposals)
	require.Nil(t, d.Document.Root.Fields, "editing must not mutate its input")
}

func TestTracfinSelectionWithoutPrefill(t *testing.T) {
	c := draftCatalog(t)
	const field = "/ds/declarant/coordonnees/nom"
	selection := models.TracfinSelection{MappingID: "m", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U"}, Bindings: []models.TracfinPathStep{{ID: "account", ParentID: "root", Table: "Account", RecordID: "A2", Link: "owner", Direction: "inverse"}, {ID: "card", ParentID: "account", Table: "Card", RecordID: "C2", Link: "account", Direction: "inverse"}}}
	d := editDraft(models.TracfinBlock{ID: "p", CatalogID: "/ds/declarant/coordonnees"})
	d.Configuration.Mappings = []models.TracfinMapping{{ID: "m", TargetField: field, Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "missing", Path: []models.TracfinPathStep{{ID: "account", ParentID: "root", Table: "Account", Link: "owner", Direction: "inverse"}, {ID: "card", ParentID: "account", Table: "Card", Link: "account", Direction: "inverse"}}}}}
	edited, err := ApplyChanges(d, []Change{{Action: "set_selection", BlockID: "p", Selection: &selection}, draftChange("set_field", "p", field, "", textInput("corrected"))}, c)
	require.NoError(t, err)
	require.Empty(t, ResolvePrefill(edited.Configuration, edited, SourceSet{}).Proposals)
	id := edited.Document.Root.Fields[field][0].ID
	b, err := json.Marshal(edited)
	require.NoError(t, err)
	var resumed models.TracfinDraft
	require.NoError(t, json.Unmarshal(b, &resumed))
	require.Equal(t, selection, *resumed.Document.Root.Selection)
	require.Equal(t, id, resumed.Document.Root.Fields[field][0].ID)
	changed := selection
	changed.Bindings = append([]models.TracfinPathStep(nil), selection.Bindings...)
	changed.Bindings[0].RecordID = "A3"
	updated, err := ApplyChanges(resumed, []Change{{Action: "set_selection", BlockID: "p", Selection: &changed}}, c)
	require.NoError(t, err)
	require.Len(t, updated.Document.Root.Selection.Bindings, 1)
	require.Equal(t, "A3", updated.Document.Root.Selection.Bindings[0].RecordID)
	require.Equal(t, id, updated.Document.Root.Fields[field][0].ID)
	require.Equal(t, "corrected", *updated.Document.Root.Fields[field][0].Text)
}

func TestTracfinChangeReferencesPreserveNumericText(t *testing.T) {
	c := draftCatalog(t)
	const field = "/ds/operations/operation/emetteurTypeEmetteur"
	d := editDraft(models.TracfinBlock{ID: "op", CatalogID: "/ds/operations/operation"})
	edited, err := ApplyChanges(d, []Change{draftChange("set_field", "op", field, "", textInput("123"))}, c)
	require.NoError(t, err)
	v := edited.Document.Root.Fields[field][0]
	require.Equal(t, "text", v.Kind)
	require.Equal(t, "123", *v.Text)
	require.Nil(t, v.Reference)
	ref := &models.TracfinFieldInput{Kind: "reference", Reference: &models.TracfinReference{BlockID: "person"}}
	edited, err = ApplyChanges(edited, []Change{draftChange("set_field", "op", field, v.ID, ref)}, c)
	require.NoError(t, err)
	v = edited.Document.Root.Fields[field][0]
	require.Equal(t, "reference", v.Kind)
	require.Equal(t, "person", v.Reference.BlockID)
	require.Nil(t, v.Text)
}

func TestTracfinDraftMainOperationsLimitAndDuplicateDecision(t *testing.T) {
	c := draftCatalog(t)
	d := editDraft(models.TracfinBlock{ID: "ops", CatalogID: "/ds/operations"})
	changes := make([]Change, 0, 60)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("op-%d", i)
		changes = append(changes, Change{Action: "add_block", ParentBlockID: "ops", BlockID: id, CatalogID: "/ds/operations/operation"}, Change{Action: "set_selection", BlockID: id, Selection: &models.TracfinSelection{MappingID: "m", DecisionID: fmt.Sprintf("decision-%d", i)}})
	}
	edited, err := ApplyChanges(d, changes, c)
	require.NoError(t, err)
	require.Len(t, edited.Document.Root.Children, 30)
	_, err = ApplyChanges(edited, []Change{{Action: "add_block", ParentBlockID: "ops", BlockID: "op-31", CatalogID: "/ds/operations/operation"}}, c)
	require.Error(t, err)
	require.Len(t, edited.Document.Root.Children, 30)
	duplicate := editDraft(models.TracfinBlock{ID: "ops", CatalogID: "/ds/operations"})
	_, err = ApplyChanges(duplicate, []Change{changes[0], changes[1], changes[2], {Action: "set_selection", BlockID: "op-1", Selection: &models.TracfinSelection{MappingID: "m", DecisionID: "decision-0"}}}, c)
	require.Error(t, err)
	require.Empty(t, duplicate.Document.Root.Children)
}

func TestTracfinChangeRepeatedScalarAttributeOwnership(t *testing.T) {
	c := draftCatalog(t)
	const scalar = "/ds/personnes/pp/etatCivil/nationalite"
	const attr = scalar + "/@type"
	d := editDraft(models.TracfinBlock{ID: "identity", CatalogID: "/ds/personnes/pp/etatCivil"})
	d, err := ApplyChanges(d, []Change{draftChange("set_field", "identity", scalar, "", textInput("FR")), draftChange("set_field", "identity", scalar, "", textInput("BE"))}, c)
	require.NoError(t, err)
	values := d.Document.Root.Fields[scalar]
	require.Len(t, values, 2)
	require.NotEqual(t, values[0].ID, values[1].ID)
	a, b := textInput("1"), textInput("2")
	a.ParentValueID = values[0].ID
	b.ParentValueID = values[1].ID
	d, err = ApplyChanges(d, []Change{draftChange("set_field", "identity", attr, "", a), draftChange("set_field", "identity", attr, "", b)}, c)
	require.NoError(t, err)
	attrs := d.Document.Root.Fields[attr]
	require.Len(t, attrs, 2)
	// Reordering persisted scalars must not change attribute ownership.
	d.Document.Root.Fields[scalar][0], d.Document.Root.Fields[scalar][1] = d.Document.Root.Fields[scalar][1], d.Document.Root.Fields[scalar][0]
	d, err = ApplyChanges(d, []Change{draftChange("clear_field", "identity", scalar, values[0].ID, nil)}, c)
	require.NoError(t, err)
	require.Len(t, d.Document.Root.Fields[attr], 1)
	require.Equal(t, values[1].ID, d.Document.Root.Fields[attr][0].ParentValueID)
}

func TestTracfinDraftManualRootsAndAtomicFailure(t *testing.T) {
	c := draftCatalog(t)
	for _, kind := range []string{"pp", "pm", "pnc"} {
		t.Run(kind, func(t *testing.T) {
			d := editDraft(models.TracfinBlock{ID: "people", CatalogID: "/ds/personnes"})
			edited, err := ApplyChanges(d, []Change{{Action: "add_block", ParentBlockID: "people", BlockID: "person", CatalogID: "/ds/personnes/" + kind}}, c)
			require.NoError(t, err)
			require.Len(t, edited.Document.Root.Children, 1)
			require.Equal(t, "person", edited.Document.Root.Children[0].ID)
			require.Nil(t, edited.Document.Root.Children[0].Selection)
			_, err = ApplyChanges(edited, []Change{{Action: "add_block", ParentBlockID: "people", BlockID: "other", CatalogID: "/ds/personnes/" + kind}, draftChange("set_field", "person", "/ds/not-a-field", "", textInput("x"))}, c)
			require.Error(t, err)
			require.Len(t, edited.Document.Root.Children, 1)
		})
	}
}

func TestTracfinDraftAmountsRemainIndependent(t *testing.T) {
	c := draftCatalog(t)
	decimal := func(s string) *models.TracfinFieldInput {
		v := models.TracfinDecimal(s)
		return &models.TracfinFieldInput{Kind: "decimal", Decimal: &v}
	}
	d := editDraft(models.TracfinBlock{ID: "ops", CatalogID: "/ds/operations"})
	changes := []Change{
		draftChange("set_field", "ops", "/ds/operations/montantTotal", "", decimal("999.00")),
		{Action: "add_block", ParentBlockID: "ops", BlockID: "main", CatalogID: "/ds/operations/operation"},
		draftChange("set_field", "main", "/ds/operations/operation/montantTotal", "", decimal("12.00")),
		{Action: "add_block", ParentBlockID: "ops", BlockID: "current", CatalogID: "/ds/operations/operationEnCours"},
		draftChange("set_field", "current", "/ds/operations/operationEnCours/montantEuro", "", decimal("7.00")),
	}
	edited, err := ApplyChanges(d, changes, c)
	require.NoError(t, err)
	require.Equal(t, models.TracfinDecimal("999.00"), *edited.Document.Root.Fields["/ds/operations/montantTotal"][0].Decimal)
	require.Equal(t, models.TracfinDecimal("12.00"), *edited.Document.Root.Children[0].Fields["/ds/operations/operation/montantTotal"][0].Decimal)
	require.Equal(t, models.TracfinDecimal("7.00"), *edited.Document.Root.Children[1].Fields["/ds/operations/operationEnCours/montantEuro"][0].Decimal)
}
