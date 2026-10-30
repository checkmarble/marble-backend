package tracfin

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

// Re-parenting an attribute onto an owner that already has it would emit an
// element with a duplicate attribute, which is not well-formed XML.
func TestTracfinAttributeOwnerCannotChange(t *testing.T) {
	c := draftCatalog(t)
	const nationality = "/ds/personnes/pp/etatCivil/nationalite"
	civil := models.TracfinBlock{ID: "civil", CatalogID: "/ds/personnes/pp/etatCivil"}
	d := editDraft(models.TracfinBlock{ID: "root", CatalogID: "/ds", Children: []models.TracfinBlock{{ID: "people", CatalogID: "/ds/personnes", Children: []models.TracfinBlock{{ID: "pp", CatalogID: "/ds/personnes/pp", Children: []models.TracfinBlock{civil}}}}}})
	d, err := ApplyChanges(d, []Change{
		{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "civil", FieldID: nationality, ValueID: "N1"}, Value: textInput("FR")},
		{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "civil", FieldID: nationality, ValueID: "N2"}, Value: textInput("BE")},
	}, c)
	// Values are created with server IDs; address them by what was stored.
	require.Error(t, err, "unknown value IDs are refused")
	d, err = ApplyChanges(d, []Change{
		{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "civil", FieldID: nationality}, Value: textInput("FR")},
		{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "civil", FieldID: nationality}, Value: textInput("BE")},
	}, c)
	require.NoError(t, err)
	owners := findBlock(&d.Document.Root, "civil").Fields[nationality]
	attr := func(owner string) *models.TracfinFieldInput {
		in := textInput("code")
		in.ParentValueID = owner
		return in
	}
	d, err = ApplyChanges(d, []Change{
		{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "civil", FieldID: nationality + "/@type", ParentValueID: owners[0].ID}, Value: attr(owners[0].ID)},
		{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "civil", FieldID: nationality + "/@type", ParentValueID: owners[1].ID}, Value: attr(owners[1].ID)},
	}, c)
	require.NoError(t, err)
	attrs := findBlock(&d.Document.Root, "civil").Fields[nationality+"/@type"]
	require.Len(t, attrs, 2)

	_, err = ApplyChanges(d, []Change{{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "civil", FieldID: nationality + "/@type", ValueID: attrs[1].ID}, Value: attr(owners[0].ID)}}, c)
	require.ErrorIs(t, err, models.BadParameterError)
	same, err := ApplyChanges(d, []Change{{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "civil", FieldID: nationality + "/@type", ValueID: attrs[1].ID}, Value: attr(owners[1].ID)}}, c)
	require.NoError(t, err, "replacing an attribute on its own owner stays allowed")
	require.Len(t, findBlock(&same.Document.Root, "civil").Fields[nationality+"/@type"], 2)

	// A stored draft that already holds the duplicate is refused by the writer.
	forged := d
	forged.Document.Root.Children[0].Children[0].Children[0].Fields[nationality+"/@type"][1].ParentValueID = owners[0].ID
	_, err = WriteXML(&bytes.Buffer{}, forged, nil, c)
	var located *XMLFieldError
	require.True(t, errors.As(err, &located), "%v", err)
	require.Equal(t, nationality+"/@type", located.Field.FieldID)
}

func TestTracfinXMLDuplicateAttributeIsNotWellFormed(t *testing.T) {
	c := draftCatalog(t)
	doc := `<ds><personnes><pp><etatCivil><nationalite type="code" type="other">FR</nationalite></etatCivil></pp></personnes></ds>`
	v := ValidateXML(strings.NewReader(doc), c)
	require.Equal(t, "failed", v.Layers["xml_syntax"].Status)
	require.True(t, task5HasCode(v.Diagnostics, "XML_SYNTAX_ERROR"))
}
