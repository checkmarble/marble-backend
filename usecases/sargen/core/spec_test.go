package core_test

import (
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
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
