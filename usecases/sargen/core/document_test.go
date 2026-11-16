package core_test

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func fullDocument() (*core.Document, core.Path) {
	d := core.NewDocument()
	p := d.AddItem("personnes.pp", "pp")
	id := uuid.New()
	d.Values[p.Child("etatCivil").Child("nomNaissance")] = core.Value{
		Value:         "DUPONT",
		Origin:        core.OriginLLM,
		Sources:       []core.SourceRef{{Object: core.ObjectRef{Table: "client", ObjectID: "1"}, Field: "name", RawValue: "dupont"}},
		RuleIDs:       []string{"r1", "r2"},
		Justification: "j",
		Confidence:    core.ConfidenceHigh,
		Reviewed:      true,
		Computation:   &core.Computation{Name: "upper", Inputs: []core.Path{"a.b", "a.c"}},
		TaskID:        &id,
	}
	d.ItemSources[p] = []core.ObjectRef{{Table: "client", ObjectID: "1"}}
	return d, p
}

func TestDocument_JSONRoundTrip(t *testing.T) {
	d, _ := fullDocument()
	n := d.AddItem(core.Path("personnes.pp[pp-1].etatCivil.nationalite"), "n")
	require.Equal(t, core.Path("personnes.pp[pp-1].etatCivil.nationalite[n-1]"), n)

	raw, err := json.Marshal(d)
	require.NoError(t, err)
	var got core.Document
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, d, &got)

	next := got.AddItem("personnes.pp", "pp")
	assert.Equal(t, core.Path("personnes.pp[pp-2]"), next)
}

func TestDocument_UnmarshalEmptyInitialisesMaps(t *testing.T) {
	for _, in := range []string{`{}`, `{"values":null,"items":null,"item_sources":null,"next_key":null}`} {
		var d core.Document
		require.NoError(t, json.Unmarshal([]byte(in), &d))
		assert.NotPanics(t, func() {
			p := d.AddItem("personnes.pp", "pp")
			d.Values[p.Child("x")] = core.Value{Value: "v"}
			d.ItemSources[p] = nil
		})
	}
}

func TestDocument_CloneIsDeep(t *testing.T) {
	d, p := fullDocument()
	vp := p.Child("etatCivil").Child("nomNaissance")
	before, err := json.Marshal(d)
	require.NoError(t, err)

	c := d.Clone()
	v := c.Values[vp]
	v.Sources[0].RawValue = "X"
	v.RuleIDs[0] = "X"
	v.Computation.Inputs[0] = "X"
	v.Computation.Name = "X"
	*v.TaskID = uuid.New()
	c.ItemSources[p][0].ObjectID = "X"
	c.NextKey["personnes.pp"] = 99
	c.Items["personnes.pp"][0] = "X"
	c.AddItem("personnes.pp", "pp")

	after, err := json.Marshal(d)
	require.NoError(t, err)
	assert.JSONEq(t, string(before), string(after))
}

func TestDocument_RemoveItemDropsNestedContainers(t *testing.T) {
	d := core.NewDocument()
	p1 := d.AddItem("personnes.pp", "pp")
	nat := p1.Child("etatCivil").Child("nationalite")
	n1 := d.AddItem(nat, "n")
	d.Values[n1.Child("code")] = core.Value{Value: "FR"}
	d.ItemSources[n1] = []core.ObjectRef{{Table: "t", ObjectID: "1"}}

	// Sibling whose key shares the textual prefix "pp-1".
	d.NextKey["personnes.pp"] = 9
	p10 := d.AddItem("personnes.pp", "pp")
	require.Equal(t, core.Path("personnes.pp[pp-10]"), p10)
	nat10 := p10.Child("etatCivil").Child("nationalite")
	n10 := d.AddItem(nat10, "n")
	d.Values[n10.Child("code")] = core.Value{Value: "DE"}
	d.Values[p10.Child("x")] = core.Value{Value: "keep"}

	d.RemoveItem(p1)

	assert.NotContains(t, d.Items, nat)
	assert.NotContains(t, d.NextKey, nat)
	assert.NotContains(t, d.Values, n1.Child("code"))
	assert.NotContains(t, d.ItemSources, n1)
	assert.Equal(t, []string{"pp-10"}, d.Items["personnes.pp"])
	assert.Equal(t, []string{"n-1"}, d.Items[nat10])
	assert.Equal(t, 1, d.NextKey[nat10])
	assert.Equal(t, "keep", d.Values[p10.Child("x")].Value)
	assert.Equal(t, "DE", d.Values[n10.Child("code")].Value)
}
