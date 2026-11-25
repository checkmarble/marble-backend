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
	assert.Equal(t, []string{"pp-2", "pp-3"}, d.ItemKeys("personnes.pp"))
	_ = b
}

func TestDocument_RemoveItemDropsValuesBelow(t *testing.T) {
	d := core.NewDocument()
	p := d.AddItem("personnes.pp", "pp")
	d.Values[p.Child("etatCivil").Child("nomNaissance")] = core.Value{Value: "DUPONT"}
	require.True(t, d.SetItemSources(p, []core.ObjectRef{{Table: "client", ObjectID: "1"}}))
	d.RemoveItem(p)
	assert.Empty(t, d.Values)
	assert.Empty(t, d.Items)
	assert.False(t, d.HasItem(p))
	assert.Nil(t, d.ItemSources(p))
}

func fullDocument() (*core.Document, core.Path) {
	d := core.NewDocument()
	p := d.AddItem("personnes.pp", "pp", core.ObjectRef{Table: "client", ObjectID: "1"})
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
	for _, in := range []string{`{}`, `{"values":null,"items":null,"next_key":null}`} {
		var d core.Document
		require.NoError(t, json.Unmarshal([]byte(in), &d))
		assert.NotPanics(t, func() {
			p := d.AddItem("personnes.pp", "pp")
			d.Values[p.Child("x")] = core.Value{Value: "v"}
			assert.True(t, d.SetItemSources(p, nil))
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
	c.Items["personnes.pp"][0].Sources[0].ObjectID = "X"
	c.NextKey["personnes.pp"] = 99
	c.Items["personnes.pp"][0].Key = "X"
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
	require.True(t, d.SetItemSources(n1, []core.ObjectRef{{Table: "t", ObjectID: "1"}}))

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
	assert.False(t, d.HasItem(n1))
	assert.Nil(t, d.ItemSources(n1))
	assert.Equal(t, []string{"pp-10"}, d.ItemKeys("personnes.pp"))
	assert.Equal(t, []string{"n-1"}, d.ItemKeys(nat10))
	assert.Equal(t, 1, d.NextKey[nat10])
	assert.Equal(t, "keep", d.Values[p10.Child("x")].Value)
	assert.Equal(t, "DE", d.Values[n10.Child("code")].Value)
}

// A JSON null document must come back with usable maps: AddItem writes to
// NextKey and Items, so a nil map there panics.
func TestDocument_UnmarshalNullInitialisesMaps(t *testing.T) {
	var d core.Document
	require.NoError(t, json.Unmarshal([]byte("null"), &d))
	assert.NotNil(t, d.Values)
	assert.NotNil(t, d.Items)
	assert.NotNil(t, d.NextKey)
	assert.NotPanics(t, func() {
		p := d.AddItem("personnes.pp", "pp")
		d.Values[p.Child("x")] = core.Value{Value: "v"}
		assert.True(t, d.SetItemSources(p, nil))
	})
}

func TestDocument_ItemKeysAndContainers(t *testing.T) {
	d := core.NewDocument()
	assert.Nil(t, d.ItemKeys("personnes.pp"))
	assert.Empty(t, d.ItemContainers())

	d.AddItem("personnes.pp", "pp")
	d.AddItem("personnes.pp", "pp")
	d.AddItem("operations.operation", "op")
	assert.Equal(t, []string{"pp-1", "pp-2"}, d.ItemKeys("personnes.pp"))
	assert.Equal(t, []core.Path{"operations.operation", "personnes.pp"}, d.ItemContainers())

	// A container emptied by RemoveItem is no longer listed.
	d.RemoveItem(d.AddItem("work.transactions", "tx"))
	assert.Equal(t, []core.Path{"operations.operation", "personnes.pp"}, d.ItemContainers())
}

func TestDocument_HasItem(t *testing.T) {
	d := core.NewDocument()
	p := d.AddItem("personnes.pp", "pp")
	assert.True(t, d.HasItem(p))
	assert.False(t, d.HasItem("personnes.pp[pp-2]"))
	assert.False(t, d.HasItem("personnes.pp"))
	assert.False(t, d.HasItem("personnes.pm[pp-1]"))
}

func TestDocument_ItemSources(t *testing.T) {
	d := core.NewDocument()
	src := []core.ObjectRef{{Table: "client", ObjectID: "1"}, {Table: "account", ObjectID: "2"}}
	p := d.AddItem("personnes.pp", "pp", src...)
	bare := d.AddItem("personnes.pp", "pp")

	got := d.ItemSources(p)
	assert.Equal(t, src, got)
	assert.Nil(t, d.ItemSources(bare))
	assert.Nil(t, d.ItemSources("personnes.pp[pp-9]"))

	// ItemSources returns a copy.
	got[0].ObjectID = "X"
	assert.Equal(t, "1", d.ItemSources(p)[0].ObjectID)
}

func TestDocument_AddItemStoresCloneOfSources(t *testing.T) {
	d := core.NewDocument()
	src := []core.ObjectRef{{Table: "client", ObjectID: "1"}}
	p := d.AddItem("personnes.pp", "pp", src...)
	src[0].ObjectID = "X"
	assert.Equal(t, []core.ObjectRef{{Table: "client", ObjectID: "1"}}, d.ItemSources(p))
}

func TestDocument_SetItemSources(t *testing.T) {
	d := core.NewDocument()
	p := d.AddItem("personnes.pp", "pp", core.ObjectRef{Table: "client", ObjectID: "1"})

	src := []core.ObjectRef{{Table: "account", ObjectID: "2"}}
	assert.True(t, d.SetItemSources(p, src))
	src[0].ObjectID = "X"
	assert.Equal(t, []core.ObjectRef{{Table: "account", ObjectID: "2"}}, d.ItemSources(p))

	assert.True(t, d.SetItemSources(p, nil))
	assert.Nil(t, d.ItemSources(p))

	// Unknown item: no-op.
	assert.False(t, d.SetItemSources("personnes.pp[pp-9]", src))
	assert.Equal(t, []string{"pp-1"}, d.ItemKeys("personnes.pp"))
}
