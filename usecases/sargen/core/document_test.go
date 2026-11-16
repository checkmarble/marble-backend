package core_test

import (
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
)

func TestDocument_ItemKeysAreNeverReused(t *testing.T) {
	d := core.NewDocument()
	a := d.AddItem("personnes.pp", "pp")
	b := d.AddItem("personnes.pp", "pp")
	d.RemoveItem(a)
	c := d.AddItem("personnes.pp", "pp")
	assert.Equal(t, core.Path("personnes.pp[pp-1]"), a)
	assert.Equal(t, core.Path("personnes.pp[pp-3]"), c)
	assert.Equal(t, []string{"pp-2", "pp-3"}, d.Items["personnes.pp"])
	_ = b
}

func TestDocument_RemoveItemDropsValuesBelow(t *testing.T) {
	d := core.NewDocument()
	p := d.AddItem("personnes.pp", "pp")
	d.Values[p.Child("etatCivil").Child("nomNaissance")] = core.Value{Value: "DUPONT"}
	d.ItemSources[p] = []core.ObjectRef{{Table: "client", ObjectID: "1"}}
	d.RemoveItem(p)
	assert.Empty(t, d.Values)
	assert.Empty(t, d.ItemSources)
}

func TestDocument_CloneIsDeep(t *testing.T) {
	d := core.NewDocument()
	p := d.AddItem("personnes.pp", "pp")
	d.Values[p.Child("etatCivil").Child("nomNaissance")] = core.Value{Value: "DUPONT"}

	c := d.Clone()
	c.Values[p.Child("etatCivil").Child("prenom")] = core.Value{Value: "Jean"}
	c.Values[p.Child("etatCivil").Child("nomNaissance")] = core.Value{Value: "MARTIN"}
	c.AddItem("personnes.pp", "pp")

	assert.Len(t, d.Values, 1)
	assert.Equal(t, "DUPONT", d.Values[p.Child("etatCivil").Child("nomNaissance")].Value)
	assert.Equal(t, []string{"pp-1"}, d.Items["personnes.pp"])
}
