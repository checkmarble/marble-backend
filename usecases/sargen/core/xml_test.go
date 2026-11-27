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

func TestWriteXML_ErrorsWriteNothing(t *testing.T) {
	var buf bytes.Buffer

	bad := xmlTestDoc("libre")
	bad.Values["dateDebut"] = core.Value{Value: "pas une date"}
	require.Error(t, core.WriteXML(xmlTestSpec(), bad, "ds", &buf))
	assert.Zero(t, buf.Len())

	dangling := xmlTestDoc("libre")
	dangling.Values["idPersonne"] = core.Value{Value: "personnes.pp[pp-9]"}
	require.Error(t, core.WriteXML(xmlTestSpec(), dangling, "ds", &buf))
	assert.Zero(t, buf.Len())
}

func TestWriteXML_AttributeAndTextNode(t *testing.T) {
	spec := &core.Spec{Root: &core.Node{Kind: core.NodeGroup, Children: []*core.Node{
		{Name: "numero", Template: "numero", Kind: core.NodeGroup, Min: 0, Max: 1, Export: true, Children: []*core.Node{
			{Name: "type", Template: "numero.type", Kind: core.NodeField, Attribute: true, Min: 1, Max: 1, Export: true, Type: core.TypeString},
			{Name: "_text", Template: "numero._text", Kind: core.NodeField, Min: 0, Max: 1, Export: true, Type: core.TypeString},
		}},
	}}}
	doc := core.NewDocument()
	doc.Values["numero.type"] = core.Value{Value: `SI"R`}
	doc.Values["numero._text"] = core.Value{Value: "123 <456>"}
	out := writeXMLForTest(t, spec, doc)
	assert.Contains(t, out, `<numero type="SI&#34;R">123 &lt;456&gt;</numero>`)
}

func TestWriteXML_EmptyMandatoryGroupIsWritten(t *testing.T) {
	spec := &core.Spec{Root: &core.Node{Kind: core.NodeGroup, Children: []*core.Node{
		{Name: "declarant", Template: "declarant", Kind: core.NodeGroup, Min: 1, Max: 1, Export: true, Children: []*core.Node{
			{Name: "nom", Template: "declarant.nom", Kind: core.NodeField, Min: 0, Max: 1, Export: true, Type: core.TypeString},
		}},
		{Name: "optionnel", Template: "optionnel", Kind: core.NodeGroup, Min: 0, Max: 1, Export: true, Children: []*core.Node{
			{Name: "x", Template: "optionnel.x", Kind: core.NodeField, Min: 0, Max: 1, Export: true, Type: core.TypeString},
		}},
	}}}
	out := writeXMLForTest(t, spec, core.NewDocument())
	assert.Contains(t, out, "<declarant/>")
	assert.NotContains(t, out, "optionnel")
}

// A field that takes either a known value or free text: ref_or_text (an item
// reference or text) and code_or_text (a code of the list or text).

func refOrTextSpec() *core.Spec {
	spec := xmlTestSpec()
	spec.Root.Children = append(spec.Root.Children, &core.Node{
		Name: "emetteur", Template: "emetteur", Kind: core.NodeField, Min: 0, Max: 1, Export: true,
		Type: core.TypeString, MaxLength: 200, RefOrText: true, RefTargets: []string{"personnes.pp[]"},
	})
	return spec
}

func codeOrTextSpec() *core.Spec {
	spec := xmlTestSpec()
	spec.CodeLists = map[string]core.CodeList{
		"pays": {Name: "pays", Codes: []core.Code{{Value: "FR", Label: "France"}, {Value: "BE", Label: "Belgique"}}},
	}
	spec.Root.Children = append(spec.Root.Children,
		&core.Node{
			Name: "pays", Template: "pays", Kind: core.NodeField, Min: 0, Max: 1, Export: true,
			Type: core.TypeCode, CodeList: "pays", MinLength: 2, MaxLength: 20, CodeOrText: true,
			TextAttribute: &core.XMLAttribute{Name: "type", Value: "other"},
		},
		&core.Node{
			Name: "nationalite", Template: "nationalite[]", Kind: core.NodeField, Min: 0, Max: 5, Export: true,
			Type: core.TypeCode, CodeList: "pays", MinLength: 2, MaxLength: 20, CodeOrText: true,
			TextAttribute: &core.XMLAttribute{Name: "type", Value: "other"},
		},
		&core.Node{
			Name: "strict", Template: "strict", Kind: core.NodeField, Min: 0, Max: 1, Export: true,
			Type: core.TypeCode, CodeList: "pays", MinLength: 2, MaxLength: 20,
		},
	)
	return spec
}

func TestWriteXML_RefOrTextReferencesItem(t *testing.T) {
	doc := xmlTestDoc("libre")
	doc.Values["emetteur"] = core.Value{Value: "personnes.pp[pp-2]", Origin: core.OriginManual}
	out := writeXMLForTest(t, refOrTextSpec(), doc)
	assert.Contains(t, out, "<emetteur>2</emetteur>")
}

func TestWriteXML_RefOrTextFreeTextVerbatim(t *testing.T) {
	doc := xmlTestDoc("libre")
	doc.Values["emetteur"] = core.Value{Value: "Société Beta & Cie", Origin: core.OriginManual}
	out := writeXMLForTest(t, refOrTextSpec(), doc)
	assert.Contains(t, out, "<emetteur>Société Beta &amp; Cie</emetteur>")
}

func TestWriteXML_CodeOrTextAttributeOnlyForFreeText(t *testing.T) {
	spec := codeOrTextSpec()
	doc := xmlTestDoc("libre")
	doc.Values["pays"] = core.Value{Value: "FR", Origin: core.OriginManual}
	out := writeXMLForTest(t, spec, doc)
	assert.Contains(t, out, "<pays>FR</pays>")

	doc.Values["pays"] = core.Value{Value: "Côte & Co", Origin: core.OriginManual}
	n1 := doc.AddItem("nationalite", "n")
	doc.Values[n1] = core.Value{Value: "BE", Origin: core.OriginManual}
	n2 := doc.AddItem("nationalite", "n")
	doc.Values[n2] = core.Value{Value: "Kosovo du Nord", Origin: core.OriginManual}
	out = writeXMLForTest(t, spec, doc)
	assert.Contains(t, out, `<pays type="other">Côte &amp; Co</pays>`)
	assert.Contains(t, out, "<nationalite>BE</nationalite>")
	assert.Contains(t, out, `<nationalite type="other">Kosovo du Nord</nationalite>`)
}
