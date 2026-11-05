package tracfin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

// task8Matrix builds one draft per catalogue path through the real editing
// commands. Block IDs equal catalogue paths so each block exists once.
type task8Matrix struct {
	t       *testing.T
	c       Catalog
	draft   models.TracfinDraft
	content []AttachmentContent
}

func newTask8Matrix(t *testing.T, c Catalog) *task8Matrix {
	return &task8Matrix{t: t, c: c, draft: models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "/ds", CatalogID: "/ds"}}}}
}

func (m *task8Matrix) apply(changes ...Change) {
	m.t.Helper()
	var err error
	m.draft, err = ApplyChanges(m.draft, changes, m.c)
	require.NoError(m.t, err)
}

func (m *task8Matrix) block(id string) string {
	m.t.Helper()
	if findBlock(&m.draft.Document.Root, id) != nil {
		return id
	}
	parent := m.block(m.c.Fields[id].Parent)
	m.apply(Change{Action: "add_block", ParentBlockID: parent, BlockID: id, CatalogID: id})
	return id
}

// owner is the document block holding a field, skipping scalar parents of attributes.
func (m *task8Matrix) owner(f Field) string {
	owner := f.Parent
	for m.c.Fields[owner].Kind == "field" {
		owner = m.c.Fields[owner].Parent
	}
	return m.block(owner)
}

func (m *task8Matrix) value(blockID, fieldID string) models.TracfinValue {
	m.t.Helper()
	values := findBlock(&m.draft.Document.Root, blockID).Fields[fieldID]
	require.NotEmpty(m.t, values, fieldID)
	return values[len(values)-1]
}

func (m *task8Matrix) set(f Field, input models.TracfinFieldInput) models.TracfinValue {
	m.t.Helper()
	owner := m.owner(f)
	if m.c.Fields[f.Parent].Kind == "field" {
		input.ParentValueID = m.set(m.c.Fields[f.Parent], m.input(m.c.Fields[f.Parent])).ID
	}
	m.apply(Change{Action: "set_field", Field: models.TracfinFieldRef{BlockID: owner, FieldID: f.ID}, Value: &input})
	return m.value(owner, f.ID)
}

// input returns a representative value of the field kind. References target
// the declared object the profile requires, in the right person context.
func (m *task8Matrix) input(f Field) models.TracfinFieldInput {
	switch f.ValueKind {
	case "boolean":
		b := true
		return models.TracfinFieldInput{Kind: "boolean", Boolean: &b}
	case "decimal":
		d := models.TracfinDecimal("1.50")
		return models.TracfinFieldInput{Kind: "decimal", Decimal: &d}
	case "reference", "text_or_reference":
		return models.TracfinFieldInput{Kind: "reference", Reference: m.reference(f)}
	}
	s := "A"
	return models.TracfinFieldInput{Kind: "text", Text: &s}
}

func (m *task8Matrix) reference(f Field) *models.TracfinReference {
	switch f.Name {
	case "idPieceJointe", "idPieceJointeDonneesConnexion":
		return &models.TracfinReference{BlockID: m.block("/ds/pieceJointe/piece")}
	case "refFinanciereEmission", "refFinanciereReception", "refFinanciere":
		personField := map[string]string{"refFinanciereEmission": "emetteurTypeEmetteur", "refFinanciereReception": "beneficiaireTypeBeneficiaire", "refFinanciere": "identEmetteur"}[f.Name]
		person := m.block("/ds/personnes/pp")
		m.set(m.c.Fields[f.Parent+"/"+personField], models.TracfinFieldInput{Kind: "reference", Reference: &models.TracfinReference{BlockID: person}})
		return &models.TracfinReference{BlockID: m.block("/ds/personnes/pp/referencesFinancieres/referenceFinanciereIban"), PersonBlockID: person}
	}
	return &models.TracfinReference{BlockID: m.block("/ds/personnes/pm")}
}

var task8Occurrence = regexp.MustCompile(`\[[0-9]+\]`)

// write serializes the draft and rereads its bytes with the independent reader.
func (m *task8Matrix) write() (XMLIndex, string) {
	m.t.Helper()
	var out bytes.Buffer
	index, err := WriteXML(&out, m.draft, m.content, m.c)
	require.NoError(m.t, err)
	reread := ValidateXML(bytes.NewReader(out.Bytes()), m.c)
	require.Equal(m.t, "passed", reread.Layers["xml_syntax"].Status, out.String())
	for _, d := range reread.Diagnostics {
		require.NotContains(m.t, []string{"UNKNOWN_XML_ELEMENT", "UNKNOWN_XML_ATTRIBUTE", "XML_ELEMENT_ORDER", "INVALID_XML_REFERENCE", "XML_SCALAR_HAS_CHILDREN"}, d.Code, "%+v\n%s", d, out.String())
	}
	return index, out.String()
}

// serialized requires the exact stable reference at the XML path of the catalogue ID.
func (m *task8Matrix) serialized(index XMLIndex, xml string, want models.TracfinFieldRef) {
	m.t.Helper()
	for path, ref := range index.Fields {
		if ref.BlockID == want.BlockID && ref.FieldID == want.FieldID && (want.ValueID == "" || ref.ValueID == want.ValueID) {
			require.Equal(m.t, want.FieldID, task8Occurrence.ReplaceAllString(path, ""))
			return
		}
	}
	m.t.Fatalf("%s is not serialized:\n%s", want.FieldID, xml)
}

func TestTracfinCoverageMatrixEveryCataloguePath(t *testing.T) {
	c := draftCatalog(t)
	counts := map[string]int{}
	for _, id := range c.OrderedFields {
		f := c.Fields[id]
		counts[f.Coverage+"/"+f.Kind]++
		t.Run(id, func(t *testing.T) {
			// Planned form contract exposed by GET /tracfin/catalog.
			require.NotEmpty(t, f.Name)
			require.NotEmpty(t, f.Model)
			m := newTask8Matrix(t, c)
			switch f.Coverage {
			case "excluded":
				require.NotEmpty(t, f.Reason)
				require.False(t, c.CompatibleParent(c.Fields[f.Parent].Parent, id) || c.CompatibleParent(f.Parent, id))
				if f.Kind == "block" {
					_, err := ApplyChanges(m.draft, []Change{{Action: "add_block", ParentBlockID: "/ds", BlockID: id, CatalogID: id}}, c)
					require.Error(t, err)
					forged := m.draft
					forged.Document.Root.Children = []models.TracfinBlock{{ID: id, CatalogID: id}}
					var out bytes.Buffer
					_, err = WriteXML(&out, forged, nil, c)
					require.NoError(t, err)
					require.NotContains(t, out.String(), "operationstdf", "a forged excluded block is never serialized")
				} else {
					s := "A"
					_, err := ApplyChanges(m.draft, []Change{{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "/ds", FieldID: id}, Value: &models.TracfinFieldInput{Kind: "text", Text: &s}}}, c)
					require.Error(t, err)
				}
			case "structural":
				require.Equal(t, "block", f.Kind)
				m.block(id)
				index, xml := m.write()
				m.serialized(index, xml, models.TracfinFieldRef{BlockID: id, FieldID: id})
			case "calculated":
				require.Equal(t, "field", f.Kind)
				owner := m.owner(f)
				if id == "/ds/pieceJointe/piece/contenu" {
					raw := []byte{0, 1, 2, 'x', 0xff}
					sum := sha256.Sum256(raw)
					attachment := models.TracfinAttachment{SourceFileID: "file", CaseID: "case", BucketName: "bucket", BlobKey: "key", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(raw))}
					findBlock(&m.draft.Document.Root, owner).Attachment = &attachment
					m.content = []AttachmentContent{{BlockID: owner, Reader: bytes.NewReader(raw), Metadata: attachment}}
				}
				// Technical identifiers are generated; the user cannot enter them.
				s := "999"
				_, err := ApplyChanges(m.draft, []Change{{Action: "set_field", Field: models.TracfinFieldRef{BlockID: owner, FieldID: id}, Value: &models.TracfinFieldInput{Kind: "text", Text: &s}}}, c)
				require.Error(t, err)
				index, xml := m.write()
				m.serialized(index, xml, models.TracfinFieldRef{BlockID: owner, FieldID: id})
			case "editable":
				require.Equal(t, "field", f.Kind)
				require.Contains(t, []string{"text", "boolean", "decimal", "reference", "text_or_reference"}, f.ValueKind)
				value := m.set(f, m.input(f))
				index, xml := m.write()
				m.serialized(index, xml, models.TracfinFieldRef{BlockID: m.owner(f), FieldID: id, ValueID: value.ID})
				if f.ValueKind == "text_or_reference" {
					// Free text, including digits, is the other representation of the same field.
					free := newTask8Matrix(t, c)
					s := "12345"
					value := free.set(f, models.TracfinFieldInput{Kind: "text", Text: &s})
					index, xml := free.write()
					free.serialized(index, xml, models.TracfinFieldRef{BlockID: free.owner(f), FieldID: id, ValueID: value.ID})
					require.True(t, strings.Contains(xml, ">12345<"))
				}
			default:
				t.Fatalf("unknown coverage %q", f.Coverage)
			}
		})
	}
	require.Equal(t, map[string]int{"editable/field": 438, "calculated/field": 41, "structural/block": 102, "excluded/block": 3, "excluded/field": 11}, counts)
}
