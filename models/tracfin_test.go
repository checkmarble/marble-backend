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
