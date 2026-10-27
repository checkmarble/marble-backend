package tracfin

import (
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
)

func TestTracfinValidationCoverageAccounting(t *testing.T) {
	c := task5Catalog(t)
	var unavailable int
	for _, r := range c.Rules {
		if r.ID == "" || r.Source == "" || r.Status == "" || r.Reason == "" {
			t.Fatalf("source rule missing provenance or status: %+v", r)
		}
		switch r.Status {
		case "unavailable":
			unavailable++
		case "excluded", "implemented_tested":
		default:
			t.Fatalf("unaccounted source rule: %+v", r)
		}
	}
	if unavailable == 0 {
		t.Fatal("test assumes untranslated source rules; revisit when all have independently verified implementation and tests")
	}
	for _, v := range []models.TracfinValidation{ValidateDraft(task5Draft(c), c), ValidateXML(strings.NewReader(`<ds/>`), c)} {
		if v.Layers["official_xsd"].Status != "unavailable" || v.Layers["official_xsd"].Reason == "" {
			t.Fatalf("official XSD must remain unavailable: %+v", v.Layers)
		}
		for _, name := range []string{"internal_structure", "references", "business"} {
			layer := v.Layers[name]
			if layer.Status == "passed" || layer.Reason == "" {
				t.Errorf("%s hides unavailable source rules: %+v", name, layer)
			}
		}
	}
}

func TestTracfinDirectorAlternativeCatalogAndEdit(t *testing.T) {
	c := task5Catalog(t)
	const base = "/ds/personnes/pm/dirigeant"
	const choice = base + "#identity"
	const id = base + "/idPersonne"
	const p = base + "/autreDirigeantP"
	const m = base + "/autreDirigeantM"
	if c.Fields[id].ValueKind != "reference" {
		t.Fatalf("director link is %q", c.Fields[id].ValueKind)
	}
	for _, field := range []string{id, p, m} {
		if c.Fields[field].Choice != choice {
			t.Errorf("%s missing documented alternative", field)
		}
	}
	person := task5Block("person", "/ds/personnes/pp")
	director := task5Block("director", base)
	d := task5Draft(c, task5Block("people", "/ds/personnes", person, task5Block("company", "/ds/personnes/pm", director)))
	var err error
	d, err = ApplyChanges(d, []Change{{Action: "set_relation", Field: models.TracfinFieldRef{BlockID: "director", FieldID: id}, Relation: &models.TracfinReference{BlockID: "person"}}, {Action: "add_block", ParentBlockID: "director", BlockID: "p", CatalogID: p}, {Action: "add_block", ParentBlockID: "director", BlockID: "m", CatalogID: m}}, c)
	if err != nil {
		t.Fatal(err)
	}
	director = d.Document.Root.Children[0].Children[1].Children[0]
	if director.ActiveBranches[choice] != m {
		t.Fatalf("adding M did not activate it: %+v", director.ActiveBranches)
	}
	name := "Old branch"
	d, err = ApplyChanges(d, []Change{{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "p", FieldID: p + "/nom"}, Value: &models.TracfinFieldInput{Kind: "text", Text: &name}}}, c)
	if err != nil {
		t.Fatal(err)
	}
	director = d.Document.Root.Children[0].Children[1].Children[0]
	if director.ActiveBranches[choice] != p {
		t.Fatalf("editing P did not reactivate it: %+v", director.ActiveBranches)
	}
}
