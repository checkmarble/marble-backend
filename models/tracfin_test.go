package models

import (
	"encoding/json"
	"testing"
)

func TestTracfinCatalogValueRoundTrip(t *testing.T) {
	empty := ""
	numericText := "123"
	no := false
	decimal := TracfinDecimal("9007199254740993.0100")
	doc := TracfinDocument{Root: TracfinBlock{ID: "root", CatalogID: "/ds", Fields: map[string][]TracfinValue{"text": {{ID: "t", Kind: "text", Text: &empty}, {ID: "t2", Kind: "text", Text: &numericText}}, "boolean": {{ID: "b", Kind: "boolean", Boolean: &no}}, "decimal": {{ID: "d", Kind: "decimal", Decimal: &decimal}}, "cleared": {{ID: "c", Kind: "cleared"}}, "reference": {{ID: "r", Kind: "reference", Reference: &TracfinReference{BlockID: "123"}}}}}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var got TracfinDocument
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	fields := got.Root.Fields
	if *fields["text"][0].Text != "" || *fields["text"][1].Text != "123" || fields["text"][0].ID != "t" {
		t.Fatal("text/repeated identity lost")
	}
	if *fields["boolean"][0].Boolean || *fields["decimal"][0].Decimal != decimal {
		t.Fatal("false or exact decimal lost")
	}
	if _, ok := fields["absent"]; ok || fields["cleared"][0].Kind != "cleared" {
		t.Fatal("absence and explicit clear conflated")
	}
	if fields["reference"][0].Reference.BlockID != "123" || fields["text"][1].Reference != nil {
		t.Fatal("numeric text and reference conflated")
	}
}

func TestTracfinCatalogRepeatedScalarAttributeOwnership(t *testing.T) {
	// Deliberately reverse attribute order and omit the third optional attribute:
	// array positions and matching attribute/value IDs cannot express ownership.
	const source = `{"id":"civil","catalog_id":"/ds/personnes/pp/etatCivil","fields":{
 "/ds/personnes/pp/etatCivil/nationalite":[
  {"id":"nationality-fr","kind":"text","text":"FR"},
  {"id":"nationality-other","kind":"text","text":"An unlisted country"},
  {"id":"nationality-de","kind":"text","text":"DE"}],
 "/ds/personnes/pp/etatCivil/nationalite/@type":[
  {"id":"attribute-other","parent_value_id":"nationality-other","kind":"text","text":"other"},
  {"id":"attribute-fr","parent_value_id":"nationality-fr","kind":"text","text":"code"}]}}`
	var block TracfinBlock
	if err := json.Unmarshal([]byte(source), &block); err != nil {
		t.Fatal(err)
	}
	const field = "/ds/personnes/pp/etatCivil/nationalite"
	check := func(want map[string]string) {
		t.Helper()
		data, err := json.Marshal(block)
		if err != nil {
			t.Fatal(err)
		}
		// Check the persisted wire contract independently of the model declaration.
		var wire struct {
			Fields map[string][]struct {
				ID            string  `json:"id"`
				ParentValueID string  `json:"parent_value_id"`
				Text          *string `json:"text"`
			} `json:"fields"`
		}
		if err = json.Unmarshal(data, &wire); err != nil {
			t.Fatal(err)
		}
		values := map[string]bool{}
		for _, v := range wire.Fields[field] {
			values[v.ID] = true
		}
		got := map[string]string{}
		for _, a := range wire.Fields[field+"/@type"] {
			if !values[a.ParentValueID] {
				t.Fatalf("attribute %s has no stable scalar owner: %q", a.ID, a.ParentValueID)
			}
			got[a.ParentValueID] = *a.Text
		}
		if len(got) != len(want) {
			t.Fatalf("attribute owners: got %v, want %v", got, want)
		}
		for id, value := range want {
			if got[id] != value {
				t.Fatalf("owner %s: got %q, want %q", id, got[id], value)
			}
		}
	}
	check(map[string]string{"nationality-fr": "code", "nationality-other": "other"})
	// Reordering values cannot move their attributes to another occurrence.
	values := block.Fields[field]
	block.Fields[field] = []TracfinValue{values[2], values[0], values[1]}
	check(map[string]string{"nationality-fr": "code", "nationality-other": "other"})
	// Represent deletion of FR and its explicitly owned attribute. The remaining
	// other attribute retains its owner even though both array positions change.
	block.Fields[field] = []TracfinValue{values[2], values[1]}
	block.Fields[field+"/@type"] = block.Fields[field+"/@type"][:1]
	check(map[string]string{"nationality-other": "other"})
}
