package tracfin

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
)

func task5Catalog(t *testing.T) Catalog {
	t.Helper()
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func task5Block(id, path string, children ...models.TracfinBlock) models.TracfinBlock {
	return models.TracfinBlock{ID: id, CatalogID: path, Fields: map[string][]models.TracfinValue{}, Children: children}
}
func task5Text(id, value string) models.TracfinValue {
	return models.TracfinValue{ID: id, Kind: "text", Text: &value}
}
func task5Draft(c Catalog, children ...models.TracfinBlock) models.TracfinDraft {
	return models.TracfinDraft{Configuration: models.TracfinConfiguration{ProfileVersion: c.Version, ProfileHash: c.Hash}, Document: models.TracfinDocument{Root: task5Block("root", "/ds", children...)}}
}
func task5HasField(ds []models.TracfinDiagnostic, field string) bool {
	for _, d := range ds {
		if d.Field != nil && d.Field.FieldID == field {
			return true
		}
	}
	return false
}
func task5HasCode(ds []models.TracfinDiagnostic, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}
func task5Civil(fields map[string][]models.TracfinValue) models.TracfinBlock {
	b := task5Block("civil", "/ds/personnes/pp/etatCivil")
	b.Fields = fields
	return task5Block("people", "/ds/personnes", task5Block("person", "/ds/personnes/pp", b))
}

func TestTracfinValidationCivilDateAndUnknownCode(t *testing.T) {
	c := task5Catalog(t)
	for _, tc := range []struct{ field, value string }{{"/ds/personnes/pp/etatCivil/dateNaissance", "29/02/2023"}, {"/ds/personnes/pp/etatCivil/nationalite", "NOT-A-COUNTRY"}} {
		t.Run(tc.field, func(t *testing.T) {
			d := task5Draft(c, task5Civil(map[string][]models.TracfinValue{tc.field: {task5Text("v", tc.value)}}))
			v := ValidateDraft(d, c)
			if !task5HasField(v.Diagnostics, tc.field) {
				t.Fatalf("missing diagnostic for %s: %+v", tc.field, v.Diagnostics)
			}
			if v.Layers["official_xsd"].Status != "unavailable" {
				t.Fatalf("official XSD: %+v", v.Layers["official_xsd"])
			}
		})
	}
}

func TestTracfinPartialExport(t *testing.T) {
	c := task5Catalog(t)
	t.Run("empty", func(t *testing.T) {
		var b bytes.Buffer
		index, err := WriteXML(&b, task5Draft(c), nil, c)
		if err != nil {
			t.Fatal(err)
		}
		var node any
		if err := xml.Unmarshal(b.Bytes(), &node); err != nil {
			t.Fatalf("invalid XML: %v: %s", err, b.String())
		}
		if len(index.Diagnostics) == 0 && len(ValidateDraft(task5Draft(c), c).Diagnostics) == 0 {
			t.Fatal("missing diagnostics for empty draft")
		}
		if strings.Contains(b.String(), "<nomNaissance>") {
			t.Fatal("invented name")
		}
	})
	t.Run("invalid and escaping", func(t *testing.T) {
		field := "/ds/personnes/pp/etatCivil/nomNaissance"
		date := "/ds/personnes/pp/etatCivil/dateNaissance"
		d := task5Draft(c, task5Civil(map[string][]models.TracfinValue{field: {task5Text("name", "A & B < C > D")}, date: {task5Text("date", "29/02/2023")}}))
		var b bytes.Buffer
		_, err := WriteXML(&b, d, nil, c)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "A &amp; B &lt; C &gt; D") || !strings.Contains(b.String(), "29/02/2023") {
			t.Fatalf("values lost or unescaped: %s", b.String())
		}
		if !task5HasField(ValidateDraft(d, c).Diagnostics, date) {
			t.Fatal("invalid date must be diagnosed")
		}
	})
	t.Run("conflict", func(t *testing.T) {
		field := "/ds/personnes/pp/etatCivil/nomNaissance"
		v := task5Text("conflict", "unresolved-secret")
		v.Origin = &models.TracfinOrigin{Resolution: "conflict"}
		d := task5Draft(c, task5Civil(map[string][]models.TracfinValue{field: {v}}))
		var b bytes.Buffer
		index, err := WriteXML(&b, d, nil, c)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(b.String(), "unresolved-secret") || !task5HasField(index.Diagnostics, field) {
			t.Fatalf("conflict not omitted and diagnosed: %s %+v", b.String(), index.Diagnostics)
		}
	})
	t.Run("forbidden character", func(t *testing.T) {
		field := "/ds/personnes/pp/etatCivil/nomNaissance"
		d := task5Draft(c, task5Civil(map[string][]models.TracfinValue{field: {task5Text("bad", "x\x01y")}}))
		var b bytes.Buffer
		if _, err := WriteXML(&b, d, nil, c); err == nil {
			t.Fatal("XML forbidden character must return an error")
		}
	})
}

func TestTracfinXMLIndependentReader(t *testing.T) {
	c := task5Catalog(t)
	for _, tc := range []struct {
		name, data string
		syntax     bool
	}{{"malformed", "<ds><personnes></ds>", true}, {"invalid date", "<ds><personnes><pp><etatCivil><dateNaissance>29/02/2023</dateNaissance></etatCivil></pp></personnes></ds>", false}} {
		t.Run(tc.name, func(t *testing.T) {
			v := ValidateXML(strings.NewReader(tc.data), c)
			if tc.syntax {
				if v.Layers["xml_syntax"].Status != "failed" {
					t.Fatalf("malformed XML accepted: %+v", v)
				}
			} else if !task5HasField(v.Diagnostics, "/ds/personnes/pp/etatCivil/dateNaissance") {
				t.Fatalf("invalid date not detected: %+v", v)
			}
			if v.Layers["official_xsd"].Status != "unavailable" {
				t.Fatalf("official XSD: %+v", v.Layers["official_xsd"])
			}
		})
	}
}

func TestTracfinValidationPieceLimit(t *testing.T) {
	c := task5Catalog(t)
	for _, n := range []int{15, 16} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			pieces := make([]models.TracfinBlock, n)
			for i := range pieces {
				pieces[i] = task5Block(fmt.Sprintf("piece-%d", i), "/ds/pieceJointe/piece")
			}
			d := task5Draft(c, task5Block("attachments", "/ds/pieceJointe", pieces...))
			v := ValidateDraft(d, c)
			found := false
			for _, diag := range v.Diagnostics {
				if diag.Field != nil && diag.Field.FieldID == "/ds/pieceJointe/piece" {
					found = true
				}
			}
			if found != (n == 16) {
				t.Fatalf("piece limit %d: %+v", n, v.Diagnostics)
			}
		})
	}
}

func TestTracfinValidationIdentityPieceCondition(t *testing.T) {
	c := task5Catalog(t)
	for _, code := range []string{"TA2", "TA3", "TA5"} {
		t.Run(code, func(t *testing.T) {
			p := task5Block("piece", "/ds/pieceJointe/piece")
			p.Fields["/ds/pieceJointe/piece/type"] = []models.TracfinValue{task5Text("type", code)}
			d := task5Draft(c, task5Block("attachments", "/ds/pieceJointe", p))
			v := ValidateDraft(d, c)
			if !task5HasField(v.Diagnostics, "/ds/pieceJointe/piece/infoPieceIdentite") {
				t.Fatalf("%s requires identity details: %+v", code, v.Diagnostics)
			}
		})
	}
}

func TestTracfinXMLRepeatedNationalityAttributes(t *testing.T) {
	c := task5Catalog(t)
	field := "/ds/personnes/pp/etatCivil/nationalite"
	attr := field + "/@type"
	a, b := task5Text("attr-a", "code"), task5Text("attr-b", "other")
	a.ParentValueID = "n1"
	b.ParentValueID = "n2"
	d := task5Draft(c, task5Civil(map[string][]models.TracfinValue{field: {task5Text("n1", "FR"), task5Text("n2", "Somewhere")}, attr: {b, a}}))
	var out bytes.Buffer
	_, err := WriteXML(&out, d, nil, c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `<nationalite type="code">FR</nationalite>`) || !strings.Contains(out.String(), `<nationalite type="other">Somewhere</nationalite>`) {
		t.Fatalf("attributes attached to wrong values: %s", out.String())
	}
}
