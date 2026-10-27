package tracfin

import (
	"bytes"
	"fmt"
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

func TestTracfinValidationOtherOperation(t *testing.T) {
	c := task5Catalog(t)
	const base = "/ds/operations/operation"
	op := task5Block("op", base)
	op.Fields[base+"/typeOperation"] = []models.TracfinValue{task5Text("type", "OT11")}
	d := task5Draft(c, task5Block("operations", "/ds/operations", op))
	v := ValidateDraft(d, c)
	if !task5HasField(v.Diagnostics, base+"/autretypeOperation") {
		t.Fatalf("OT11 requires description: %+v", v.Diagnostics)
	}
}

func TestTracfinValidationCombinedFinancialSupportLimit(t *testing.T) {
	c := task5Catalog(t)
	const base = "/ds/personnes/pp/referencesFinancieres"
	for _, n := range []int{20, 21} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			refs := task5Block("refs", base)
			for i := 0; i < n; i++ {
				kind := "referenceFinanciereIban"
				if i%2 == 1 {
					kind = "referenceFinanciereNumeroCarteBancaire"
				}
				refs.Children = append(refs.Children, task5Block(fmt.Sprintf("support-%d", i), base+"/"+kind))
			}
			d := task5Draft(c, task5Block("people", "/ds/personnes", task5Block("person", "/ds/personnes/pp", refs)))
			v := ValidateDraft(d, c)
			found := false
			for _, diag := range v.Diagnostics {
				if diag.Field != nil && diag.Field.FieldID == base {
					found = true
				}
			}
			if found != (n == 21) {
				t.Fatalf("combined support limit %d: %+v", n, v.Diagnostics)
			}
		})
	}
}
