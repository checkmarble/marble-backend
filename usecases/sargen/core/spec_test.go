package core_test

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSpec() *core.Spec {
	return &core.Spec{Root: &core.Node{Children: []*core.Node{
		{Name: "organisme", Template: "organisme", Kind: core.NodeGroup, Min: 1, Max: 1, Children: []*core.Node{
			{Name: "profession", Template: "organisme.profession", Kind: core.NodeField, Min: 1, Max: 1},
		}},
		{Name: "personnes", Template: "personnes", Kind: core.NodeGroup, Min: 1, Max: 1, Children: []*core.Node{
			{Name: "pp", Template: "personnes.pp[]", Kind: core.NodeGroup, Min: 0, Max: -1, Children: []*core.Node{
				{Name: "nom", Template: "personnes.pp[].nom", Kind: core.NodeField, Min: 1, Max: 1},
				{Name: "prenom", Template: "personnes.pp[].prenom", Kind: core.NodeField, Min: 0, Max: 1},
			}},
		}},
	}}}
}

func TestSpec_NodeAt(t *testing.T) {
	s := testSpec()
	assert.Equal(t, "profession", s.NodeAt("organisme.profession").Name)
	assert.Equal(t, "pp", s.NodeAt("personnes.pp[]").Name)
	assert.Equal(t, "prenom", s.NodeAt("personnes.pp[].prenom").Name)
	assert.Nil(t, s.NodeAt("nope"))
}

func TestSpec_Leaves(t *testing.T) {
	s := testSpec()
	d := core.NewDocument()
	d.AddItem("personnes.pp", "pp")
	d.AddItem("personnes.pp", "pp")
	assert.Equal(t, []core.Path{
		"organisme.profession",
		"personnes.pp[pp-1].nom", "personnes.pp[pp-1].prenom",
		"personnes.pp[pp-2].nom", "personnes.pp[pp-2].prenom",
	}, s.Leaves("", d))
	assert.Equal(t, []core.Path{"personnes.pp[pp-2].nom", "personnes.pp[pp-2].prenom"}, s.Leaves("personnes.pp[pp-2]", d))
	assert.Len(t, s.Leaves("personnes.pp", d), 4)
}

func TestEffectiveDefaults(t *testing.T) {
	assert.Equal(t, core.SeverityBlocking, core.Rule{}.EffectiveSeverity())
	assert.Equal(t, core.SeverityWarning, core.Rule{Severity: core.SeverityWarning}.EffectiveSeverity())
	assert.Equal(t, core.NatureCopied, (&core.Node{}).EffectiveNature())
	assert.Equal(t, core.NatureJudgment, (&core.Node{Nature: core.NatureJudgment}).EffectiveNature())
}

func TestNode_MaxZeroIsNotRepeatable(t *testing.T) {
	s := &core.Spec{Root: &core.Node{Children: []*core.Node{
		{Name: "a", Template: "a", Kind: core.NodeField, Max: 0},
	}}}
	assert.Equal(t, []core.Path{"a"}, s.Leaves("", core.NewDocument()))
}

func TestSpec_JSONCarriesExportIDAndDefaults(t *testing.T) {
	raw := `{"root":{"name":"ds","template":"","kind":"group","min":1,"max":1,"export":true,"children":[` +
		`{"name":"id","template":"id","kind":"field","min":1,"max":1,"export":true,"export_id":true}]},` +
		`"defaults":{"id":"{case_id}"}}`

	var s core.Spec
	require.NoError(t, json.Unmarshal([]byte(raw), &s))
	assert.True(t, s.NodeAt("id").ExportID)
	assert.Equal(t, "{case_id}", s.Defaults["id"])
}
