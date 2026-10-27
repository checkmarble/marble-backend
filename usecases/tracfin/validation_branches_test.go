package tracfin

import (
	"bytes"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
)

func TestTracfinInactiveBranchExport(t *testing.T) {
	c := task5Catalog(t)
	const base = "/ds/personnes/pm/dirigeant"
	p := task5Block("p", base+"/autreDirigeantP")
	p.Fields[base+"/autreDirigeantP/nom"] = []models.TracfinValue{task5Text("name-p", "Physical Name")}
	m := task5Block("m", base+"/autreDirigeantM")
	m.Fields[base+"/autreDirigeantM/denominationSociale"] = []models.TracfinValue{task5Text("name-m", "Company Name")}
	director := task5Block("director", base, p, m)
	director.ActiveBranches = map[string]string{base + "#identity": base + "/autreDirigeantM"}
	d := task5Draft(c, task5Block("people", "/ds/personnes", task5Block("company", "/ds/personnes/pm", director)))
	var out bytes.Buffer
	index, err := WriteXML(&out, d, nil, c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Physical Name") || !strings.Contains(out.String(), "Company Name") || !task5HasCode(index.Diagnostics, "INACTIVE_BRANCH_DATA_NOT_EXPORTED") {
		t.Fatalf("inactive branch not omitted and diagnosed: %s %+v", out.String(), index.Diagnostics)
	}
	d.Document.Root.Children[0].Children[0].Children[0].ActiveBranches[base+"#identity"] = base + "/autreDirigeantP"
	out.Reset()
	index, err = WriteXML(&out, d, nil, c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Physical Name") || strings.Contains(out.String(), "Company Name") || !task5HasCode(index.Diagnostics, "INACTIVE_BRANCH_DATA_NOT_EXPORTED") {
		t.Fatalf("branch not restored: %s %+v", out.String(), index.Diagnostics)
	}
}

func TestTracfinExportDanglingReference(t *testing.T) {
	c := task5Catalog(t)
	const field = "/ds/personnes/pm/dirigeant/idPersonne"
	director := task5Block("director", "/ds/personnes/pm/dirigeant")
	director.Fields[field] = []models.TracfinValue{{ID: "link", Kind: "reference", Reference: &models.TracfinReference{BlockID: "missing"}}}
	d := task5Draft(c, task5Block("people", "/ds/personnes", task5Block("company", "/ds/personnes/pm", director)))
	var out bytes.Buffer
	index, err := WriteXML(&out, d, nil, c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "<idPersonne>") || !task5HasField(index.Diagnostics, field) {
		t.Fatalf("dangling reference not omitted and diagnosed: %s %+v", out.String(), index.Diagnostics)
	}
}
