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

func TestTracfinValidationScalarConstraints(t *testing.T) {
	c := task5Catalog(t)
	t.Run("maximum length", func(t *testing.T) {
		field := "/ds/personnes/pp/etatCivil/nomNaissance"
		d := task5Draft(c, task5Civil(map[string][]models.TracfinValue{field: {task5Text("v", strings.Repeat("a", 201))}}))
		if !task5HasField(ValidateDraft(d, c).Diagnostics, field) {
			t.Fatal("201-character name accepted")
		}
	})
	for _, tc := range []struct{ field, value, kind string }{{"nombreOperations", "0", "text"}, {"nombreOperations", "1000000", "text"}, {"montantTotal", "1234567890123", "decimal"}} {
		t.Run(tc.field+tc.value, func(t *testing.T) {
			field := "/ds/operations/operation/" + tc.field
			op := task5Block("op", "/ds/operations/operation")
			if tc.kind == "decimal" {
				n := models.TracfinDecimal(tc.value)
				op.Fields[field] = []models.TracfinValue{{ID: "v", Kind: "decimal", Decimal: &n}}
			} else {
				op.Fields[field] = []models.TracfinValue{task5Text("v", tc.value)}
			}
			d := task5Draft(c, task5Block("ops", "/ds/operations", op))
			if !task5HasField(ValidateDraft(d, c).Diagnostics, field) {
				t.Fatalf("invalid %s accepted", tc.value)
			}
		})
	}
}

func TestTracfinXMLIndependentStructure(t *testing.T) {
	c := task5Catalog(t)
	for _, tc := range []struct{ name, doc string }{{"unknown attribute", `<ds unexpected="x"></ds>`}, {"wrong order", `<ds><operations></operations><personnes></personnes></ds>`}, {"too many elements", `<ds><personnes><pp><etatCivil><nomNaissance>A</nomNaissance><nomNaissance>B</nomNaissance></etatCivil></pp></personnes></ds>`}} {
		t.Run(tc.name, func(t *testing.T) {
			v := ValidateXML(strings.NewReader(tc.doc), c)
			if v.Layers["internal_structure"].Status != "failed" {
				t.Fatalf("invalid structure accepted: %+v", v)
			}
		})
	}
}
