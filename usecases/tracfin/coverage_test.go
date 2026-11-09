package tracfin

import (
	"github.com/checkmarble/marble-backend/models"
	"strings"
	"testing"
)

func TestCoverageOnlyBlocksActiveRelevantRules(t *testing.T) {
	c := task5Catalog(t)
	c.Rules = []SourceRule{{ID: "missing", Kind: "catalog", Status: "unavailable", Fields: []string{"/ds/analyse"}}}
	root := task5Block("root", "/ds")
	v := models.TracfinValidation{Layers: map[string]models.TracfinValidationLayer{"internal_structure": {Status: "passed"}, "business": {Status: "passed"}, "references": {Status: "passed"}}}
	validateRuleCoverage(&v, root, c)
	if v.Layers["business"].Status == "passed" {
		t.Fatal("required missing field coverage must be incomplete")
	}
	c.Rules[0].Fields = []string{"/ds/pieceJointe/piece/type_mime"}
	v.Layers["business"] = models.TracfinValidationLayer{Status: "passed"}
	validateRuleCoverage(&v, root, c)
	if v.Layers["business"].Status != "passed" {
		t.Fatal("absent optional section must not block")
	}
}
func TestDraftRejectsUnknownBlocksAndFields(t *testing.T) {
	c := task5Catalog(t)
	root := task5Block("root", "/ds", task5Block("bad", "/ds/invalid"))
	root.Fields["/ds/unknown"] = []models.TracfinValue{task5Text("v", "x")}
	v := ValidateDraft(task5Draft(c, root.Children...), c)
	// task5Draft creates the root, so add the unknown field separately.
	d := task5Draft(c, root.Children...)
	d.Document.Root.Fields = root.Fields
	v = ValidateDraft(d, c)
	if !task5HasCode(v.Diagnostics, "UNKNOWN_DOCUMENT_FIELD") || !task5HasCode(v.Diagnostics, "UNKNOWN_DOCUMENT_BLOCK") {
		t.Fatalf("missing structural errors: %+v", v.Diagnostics)
	}
}

func TestCoverageCompatibleProfile(t *testing.T) {
	c := task5Catalog(t)
	old := "00d222c12baef60e12e35306660a5472d7d5ad88aa736c980bf4a1e4eeb9db68"
	if !c.CompatibleProfile(c.Version, old) || c.CompatibleProfile(c.Version, "unknown") || c.CompatibleProfile("other", old) {
		t.Fatal("legacy hash compatibility is not scoped to the same version")
	}
}

func TestConditionalDocumentationCoverageRegistry(t *testing.T) {
	c := task5Catalog(t)
	required := map[int]bool{}
	for _, line := range []int{665, 853, 861, 870, 890, 898, 907, 988, 995, 1002, 1009, 1016, 1023, 1030, 1037, 1221, 1273, 1280, 1287, 1294, 1301, 1308, 1315, 1322, 1329, 1360, 1394, 1427, 1435, 1545, 1553, 1560, 1568, 1576, 1583, 1633, 1685, 1695, 1702, 1709, 1716, 1746, 1753, 1813} {
		required[line] = true
	}
	for _, r := range c.Rules {
		if r.Kind == "documentation" && required[r.Line] {
			if r.Status != "implemented_tested" || r.Validator != "conditional_business" {
				t.Errorf("conditional rule %s missing validator ownership", r.ID)
			}
			delete(required, r.Line)
		}
		if r.Status == "implemented_generic" && r.Validator != "structure" && r.Validator != "scalar" && r.Validator != "catalog" {
			t.Errorf("%s missing generic validator", r.ID)
		}
	}
	if len(required) > 0 {
		t.Fatalf("conditional rules missing: %v", required)
	}
}
func TestGenericTotalDigitsIgnoresInsignificantZeroes(t *testing.T) {
	report := models.TracfinValidation{Layers: map[string]models.TracfinValidationLayer{"business": {Status: "passed"}}}
	validateScalar(&report, Field{Base: "xs:decimal", Constraints: []Constraint{{Kind: "totalDigits", Value: "1"}}}, "0.00100", models.TracfinFieldRef{})
	if len(report.Diagnostics) != 0 {
		t.Fatalf("insignificant zeroes counted: %+v", report.Diagnostics)
	}
}

func TestXMLCountryOtherSkipsCodeCatalogForItsOwnValue(t *testing.T) {
	c := task5Catalog(t)
	v := ValidateXML(strings.NewReader(`<ds><personnes><pp><etatCivil><nationalite type="other">Somewhere</nationalite></etatCivil></pp></personnes></ds>`), c)
	for _, d := range v.Diagnostics {
		if d.Code == "UNKNOWN_CODE" && d.Field != nil && d.Field.FieldID == "/ds/personnes/pp/etatCivil/nationalite" {
			t.Fatalf("other country was treated as a code: %+v", d)
		}
	}
}
