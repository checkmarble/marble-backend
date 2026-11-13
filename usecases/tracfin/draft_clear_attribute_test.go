package tracfin

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

func TestTracfinChangeClearAbsentScalarAttribute(t *testing.T) {
	c := draftCatalog(t)
	const scalar = "/ds/personnes/pp/etatCivil/nationalite"
	const attribute = scalar + "/@type"
	const otherScalar = "/ds/personnes/pp/etatCivil/paysNaissance"
	const otherAttr = otherScalar + "/@type"
	nationality, country, other, source := "FR", "FR", "code", "code"
	d := editDraft(models.TracfinBlock{ID: "identity", CatalogID: "/ds/personnes/pp/etatCivil", Fields: map[string][]models.TracfinValue{
		scalar:      {{ID: "nationality", Kind: "text", Text: &nationality}},
		otherScalar: {{ID: "birth-country", Kind: "text", Text: &country}},
		otherAttr:   {{ID: "other", ParentValueID: "birth-country", Kind: "text", Text: &other}},
	}})
	cfg := models.TracfinConfiguration{Mappings: []models.TracfinMapping{{ID: "m", TargetField: attribute, Codes: []models.TracfinCodeMapping{{Source: "code", Target: "code"}}, Source: models.TracfinMappingSource{Kind: "settings", Field: attribute}}}, Settings: models.TracfinDocument{Root: models.TracfinBlock{Fields: map[string][]models.TracfinValue{attribute: {{Kind: "text", Text: &source}}}}}}
	require.Len(t, ResolvePrefill(cfg, d, SourceSet{}).Proposals, 1)
	clear := func(owner string) Change {
		return Change{Action: "clear_field", Field: models.TracfinFieldRef{BlockID: "identity", FieldID: attribute}, Value: &models.TracfinFieldInput{ParentValueID: owner}}
	}
	for _, owner := range []string{"", "missing", "birth-country"} {
		t.Run("invalid owner "+owner, func(t *testing.T) {
			_, err := ApplyChanges(d, []Change{clear(owner)}, c)
			require.Error(t, err)
			require.Empty(t, d.Document.Root.Fields[attribute])
		})
	}
	got, err := ApplyChanges(d, []Change{clear("nationality")}, c)
	require.NoError(t, err)
	values := got.Document.Root.Fields[attribute]
	require.Len(t, values, 1)
	require.NotEmpty(t, values[0].ID)
	require.Equal(t, "cleared", values[0].Kind)
	require.Equal(t, "nationality", values[0].ParentValueID)
	require.Equal(t, "manual", values[0].Origin.Source)
	require.Equal(t, "cleared", values[0].Origin.Resolution)
	require.Equal(t, d.Document.Root.Fields[otherAttr], got.Document.Root.Fields[otherAttr])
	require.Empty(t, ResolvePrefill(cfg, got, SourceSet{}).Proposals)
}
