package core_test

import (
	"bytes"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func refOrTextSpec() *core.Spec {
	spec := xmlTestSpec()
	spec.Root.Children = append(spec.Root.Children, &core.Node{
		Name: "emetteur", Template: "emetteur", Kind: core.NodeField, Min: 0, Max: 1, Export: true,
		Type: core.TypeString, MaxLength: 200, RefOrText: true, RefTargets: []string{"personnes.pp[]"},
	})
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

func TestValidate_RefOrText(t *testing.T) {
	spec := refOrTextSpec()
	doc := xmlTestDoc("libre")

	doc.Values["emetteur"] = core.Value{Value: "personnes.pp[pp-9]", Origin: core.OriginManual}
	var codes []string
	for _, i := range core.Validate(spec, doc) {
		codes = append(codes, i.Code)
	}
	assert.Contains(t, codes, core.CodeDanglingRef)

	doc.Values["emetteur"] = core.Value{Value: "personnes.pp[pp-2]", Origin: core.OriginManual}
	for _, i := range core.Validate(spec, doc) {
		assert.NotEqual(t, "emetteur", string(i.Path), i.Code)
	}

	doc.Values["emetteur"] = core.Value{Value: "Société Beta", Origin: core.OriginManual}
	for _, i := range core.Validate(spec, doc) {
		assert.NotEqual(t, "emetteur", string(i.Path), i.Code)
	}
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
