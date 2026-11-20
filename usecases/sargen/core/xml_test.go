package core_test

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// xmlTestSpec is a small hand-made spec: root "ds", a date and a string field,
// a working group, a repeatable person group with an export identifier, and a
// reference field targeting the person template.
func xmlTestSpec() *core.Spec {
	return &core.Spec{
		Root: &core.Node{
			Template: "",
			Kind:     core.NodeGroup,
			Children: []*core.Node{
				{
					Name: "dateDebut", Template: "dateDebut", Kind: core.NodeField,
					Min: 1, Max: 1, Export: true, Type: core.TypeDate,
				},
				{
					Name: "libelle", Template: "libelle", Kind: core.NodeField,
					Min: 0, Max: 1, Export: true, Type: core.TypeString,
				},
				{
					Name: "work", Template: "work", Kind: core.NodeGroup,
					Min: 0, Max: 1, Export: false,
					Children: []*core.Node{
						{
							Name: "note", Template: "work.note", Kind: core.NodeField,
							Min: 0, Max: 1, Export: false, Type: core.TypeString,
						},
					},
				},
				{
					Name: "personnes", Template: "personnes", Kind: core.NodeGroup,
					Min: 0, Max: 1, Export: true,
					Children: []*core.Node{
						{
							Name: "pp", Template: "personnes.pp[]", Kind: core.NodeGroup,
							Min: 0, Max: -1, Export: true,
							Children: []*core.Node{
								{
									Name: "id", Template: "personnes.pp[].id", Kind: core.NodeField,
									Min: 1, Max: 1, Export: true, ExportID: true, Type: core.TypeString,
								},
								{
									Name: "nom", Template: "personnes.pp[].nom", Kind: core.NodeField,
									Min: 0, Max: 1, Export: true, Type: core.TypeString,
								},
							},
						},
					},
				},
				{
					Name: "idPersonne", Template: "idPersonne", Kind: core.NodeField,
					Min: 0, Max: 1, Export: true, Type: core.TypeRef,
					RefTargets: []string{"personnes.pp[]"},
				},
			},
		},
	}
}

// xmlTestDoc returns a document with one date, one string, one working value,
// and two persons (pp-1, pp-2).
func xmlTestDoc(libelle string) *core.Document {
	doc := core.NewDocument()
	doc.Values["dateDebut"] = core.Value{Value: "2026-11-01", Origin: core.OriginManual}
	doc.Values["libelle"] = core.Value{Value: libelle, Origin: core.OriginManual}
	doc.Values["work.note"] = core.Value{Value: "note de travail", Origin: core.OriginManual}
	p1 := doc.AddItem("personnes.pp", "pp")
	doc.Values[p1.Child("nom")] = core.Value{Value: "DUPONT", Origin: core.OriginManual}
	p2 := doc.AddItem("personnes.pp", "pp")
	doc.Values[p2.Child("nom")] = core.Value{Value: "MARTIN", Origin: core.OriginManual}
	return doc
}

func writeXMLForTest(t *testing.T, spec *core.Spec, doc *core.Document) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, core.WriteXML(spec, doc, "ds", &buf))
	return buf.String()
}

func TestWriteXML_DatesInFrenchFormat(t *testing.T) {
	out := writeXMLForTest(t, xmlTestSpec(), xmlTestDoc("libre"))
	assert.Contains(t, out, "<dateDebut>01/11/2026</dateDebut>")
	assert.NotContains(t, out, "2026-11-01")
}

func TestWriteXML_EscapesSpecialCharacters(t *testing.T) {
	const original = `<&>"`
	out := writeXMLForTest(t, xmlTestSpec(), xmlTestDoc("analyse "+original))

	type ds struct {
		Libelle string `xml:"libelle"`
	}
	var parsed ds
	require.NoError(t, xml.Unmarshal([]byte(out), &parsed), "output must be well-formed XML:\n%s", out)
	assert.Equal(t, "analyse "+original, parsed.Libelle)
}

func TestWriteXML_SkipsWorkingNodes(t *testing.T) {
	out := writeXMLForTest(t, xmlTestSpec(), xmlTestDoc("libre"))
	assert.NotContains(t, out, "<work")
	assert.NotContains(t, out, "note de travail")
}

func TestWriteXML_RefBecomesTargetId(t *testing.T) {
	doc := xmlTestDoc("libre")
	doc.Values["idPersonne"] = core.Value{Value: "personnes.pp[pp-2]", Origin: core.OriginManual}

	out := writeXMLForTest(t, xmlTestSpec(), doc)

	type person struct {
		ID  string `xml:"id"`
		Nom string `xml:"nom"`
	}
	type ds struct {
		Persons []person `xml:"personnes>pp"`
		Ref     string   `xml:"idPersonne"` //nolint:tagliatelle
	}
	var parsed ds
	require.NoError(t, xml.Unmarshal([]byte(out), &parsed), "output must be well-formed XML:\n%s", out)

	require.Len(t, parsed.Persons, 2)
	assert.Equal(t, "1", parsed.Persons[0].ID)
	assert.Equal(t, "DUPONT", parsed.Persons[0].Nom)
	assert.Equal(t, "2", parsed.Persons[1].ID)
	assert.Equal(t, "MARTIN", parsed.Persons[1].Nom)

	// The reference points at pp-2, the second person, whose XML id is "2".
	assert.Equal(t, "2", parsed.Ref)
}

func TestWriteXML_StripsControlCharacters(t *testing.T) {
	out := writeXMLForTest(t, xmlTestSpec(), xmlTestDoc("ab\x01cd"))
	assert.NotContains(t, out, "\x01")

	type ds struct {
		Libelle string `xml:"libelle"`
	}
	var parsed ds
	require.NoError(t, xml.Unmarshal([]byte(out), &parsed), "output must be well-formed XML:\n%s", out)
	assert.Equal(t, "abcd", parsed.Libelle)
}

func TestWriteXML_WritesHeader(t *testing.T) {
	out := writeXMLForTest(t, xmlTestSpec(), xmlTestDoc("libre"))
	first := strings.SplitN(out, "\n", 2)[0]
	assert.Equal(t, `<?xml version="1.0" encoding="UTF-8"?>`, strings.TrimSpace(first))
}
