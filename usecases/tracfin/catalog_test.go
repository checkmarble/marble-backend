package tracfin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"github.com/checkmarble/marble-backend/models"
	"io"
	"os"
	"strings"
	"testing"
)

func TestTracfinCatalogCoverage(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Fields) != 595 {
		t.Fatalf("coverage: got %d paths, want 595", len(c.Fields))
	}
	for _, person := range []string{"pp", "pm", "pnc"} {
		count := 0
		for _, f := range c.Fields {
			if f.Parent == "/ds/personnes/"+person+"/referencesFinancieres" && f.Kind == "block" {
				count++
			}
		}
		if count != 12 {
			t.Fatalf("%s supports: %d", person, count)
		}
	}
	for _, f := range c.Fields {
		if f.Coverage == "" || f.Model == "" {
			t.Fatalf("uncovered %s", f.ID)
		}
	}
	if c.Fields["/ds/operationstdf"].Coverage != "excluded" {
		t.Fatal("TDF must be excluded")
	}
	if c.CodeStatus("typesdedocuments.csv", "TA17") != "excluded" {
		t.Fatal("TA17 must be excluded")
	}
	if c.Fields["/ds/operations/operation/emetteurTypeEmetteur"].ValueKind != "text_or_reference" {
		t.Fatal("missing explicit reference variant")
	}
	if c.Fields["/ds/operations/operationEnCours/montantEuro"].Currency != "EUR" {
		t.Fatal("missing proven target currency")
	}
	if len(c.Rules) == 0 {
		t.Fatal("missing independent source rules")
	}
	for _, r := range c.Rules {
		if r.Source == "" || r.Status == "" {
			t.Fatal("rule without source/status")
		}
	}
}

func TestTracfinUnknownCatalogCode(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if c.CodeStatus("professions.csv", "TPI1") != "known" {
		t.Fatal("raw source code missing")
	}
	if c.CodeStatus("professions.csv", "invented") != "unknown" {
		t.Fatal("unknown accepted")
	}
	if c.CodeStatus("missing.csv", "anything") != "unavailable" {
		t.Fatal("missing catalog is not unavailable")
	}
}

func TestTracfinConfiguration(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	field := "/ds/organisme/profession"
	cfg := models.TracfinConfiguration{Mappings: []models.TracfinMapping{{ID: "m1", TargetField: field, Source: models.TracfinMappingSource{Kind: "ingested", EntityType: "Person", Table: "person", Field: "profession"}, Codes: []models.TracfinCodeMapping{{Source: "client profession", Target: "TPI1"}}}}}
	if d := ValidateConfiguration(cfg, c); len(d) != 0 {
		t.Fatalf("valid mapping: %+v", d)
	}
	cfg.Mappings[0].Codes = append(cfg.Mappings[0].Codes, models.TracfinCodeMapping{Source: "client profession", Target: "TPI9"})
	if d := ValidateConfiguration(cfg, c); len(d) != 1 || d[0].Code != "CONTRADICTORY_CODE_MAPPING" {
		t.Fatalf("contradictory: %+v", d)
	}
	cfg.Mappings[0].Codes = []models.TracfinCodeMapping{{Source: "", Target: "not-a-code"}}
	if d := ValidateConfiguration(cfg, c); len(d) != 2 {
		t.Fatalf("invalid source/target: %+v", d)
	}
	cfg.Mappings[0].TargetField = "/ds/operationstdf"
	if d := ValidateConfiguration(cfg, c); len(d) == 0 {
		t.Fatal("excluded target accepted")
	}
}

func TestTracfinCatalogViewerParity(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/modele-viewer.json")
	if err != nil {
		t.Fatal(err)
	}
	var viewer struct {
		Nodes []struct {
			Path string `json:"path"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(data, &viewer); err != nil {
		t.Fatal(err)
	}
	if len(viewer.Nodes) != len(c.Fields) {
		t.Fatal("viewer/profile size differs")
	}
	for _, n := range viewer.Nodes {
		if _, ok := c.Fields[n.Path]; !ok {
			t.Fatalf("missing viewer path %s", n.Path)
		}
	}
	for _, id := range []string{"/ds/personnes/pp/id", "/ds/personnes/pm/id", "/ds/personnes/pnc/id"} {
		if c.Fields[id].Coverage != "calculated" {
			t.Fatalf("XML identity must be server-calculated: %s", id)
		}
	}
	if c.CompatibleParent("/ds/personnes/pm", "/ds/personnes/pp/etatCivil") {
		t.Fatal("cross-parent field accepted")
	}
	if !c.CompatibleParent("/ds/personnes/pp", "/ds/personnes/pp/etatCivil") {
		t.Fatal("valid parent rejected")
	}
}

func TestTracfinCatalogSourceRules(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	// Rules are enumerated from source, not only fields used in happy-path fixtures.
	data, err := profileFiles.ReadFile("profile/sources/financier-v1.4.0.xsd")
	if err != nil {
		t.Fatal(err)
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	docs, facets := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if start, ok := token.(xml.StartElement); ok {
			switch start.Name.Local {
			case "documentation":
				docs++
			case "pattern", "minLength", "maxLength", "totalDigits", "fractionDigits", "enumeration", "minInclusive", "maxInclusive":
				facets++
			}
		}
	}
	gotDocs, gotFacets := 0, 0
	for _, r := range c.Rules {
		switch r.Kind {
		case "documentation":
			gotDocs++
		case "pattern", "minLength", "maxLength", "totalDigits", "fractionDigits", "enumeration", "minInclusive", "maxInclusive":
			gotFacets++
		}
		if r.Status == "implemented" || r.Status == "tested" {
			t.Fatal("Task1 must not claim unimplemented draft validation")
		}
	}
	if docs != gotDocs || facets != gotFacets {
		t.Fatalf("source inventory: docs %d/%d facets %d/%d", gotDocs, docs, gotFacets, facets)
	}
}

func TestTracfinConfigurationSourceAndCurrency(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		mapping models.TracfinMapping
		want    string
	}{
		{"missing entity", models.TracfinMapping{ID: "m", TargetField: "/ds/organisme/libelle", Source: models.TracfinMappingSource{Kind: "ingested", Table: "partner", Field: "name"}}, "INVALID_ENTITY_TYPE"},
		{"partner", models.TracfinMapping{ID: "m", TargetField: "/ds/organisme/libelle", Source: models.TracfinMappingSource{Kind: "ingested", EntityType: "Partner", Table: "partner", Field: "name"}}, "INVALID_ENTITY_TYPE"},
		{"missing catalog", models.TracfinMapping{ID: "m", TargetField: "/ds/pieceJointe/piece/type_mime", Source: models.TracfinMappingSource{Kind: "settings", Field: "mime"}, Codes: []models.TracfinCodeMapping{{Source: "PDF", Target: "application/pdf"}}}, "CATALOG_UNAVAILABLE"},
		{"excluded code", models.TracfinMapping{ID: "m", TargetField: "/ds/pieceJointe/piece/type", Source: models.TracfinMappingSource{Kind: "settings", Field: "document_type"}, Codes: []models.TracfinCodeMapping{{Source: "tdf", Target: "TA17"}}}, "EXCLUDED_CATALOG_CODE"},
		{"unknown currency evidence", models.TracfinMapping{ID: "m", TargetField: "/ds/operations/operation/montantTotal", Source: models.TracfinMappingSource{Kind: "decision_snapshot", HistoricalType: "transaction", Field: "amount"}}, "MISSING_TARGET_CURRENCY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ds := ValidateConfiguration(models.TracfinConfiguration{Mappings: []models.TracfinMapping{tc.mapping}}, c)
			for _, d := range ds {
				if d.Code == tc.want {
					return
				}
			}
			t.Fatalf("want %s, got %+v", tc.want, ds)
		})
	}
	cfg := models.TracfinConfiguration{Mappings: []models.TracfinMapping{{ID: "m", TargetField: "/ds/operations/operationEnCours/montantEuro", Source: models.TracfinMappingSource{Kind: "decision_snapshot", HistoricalType: "deleted_transaction", Field: "amount"}, Unit: "minor", TargetCurrency: "EUR", Currency: models.TracfinCurrencyEvidence{Constant: "EUR"}}}}
	if ds := ValidateConfiguration(cfg, c); len(ds) != 0 {
		t.Fatalf("EUR cents historical snapshot mapping: %+v", ds)
	}
	cfg.Mappings[0].Currency.Constant = "USD"
	ds := ValidateConfiguration(cfg, c)
	if len(ds) != 1 || ds[0].Code != "INCOMPATIBLE_SOURCE_CURRENCY" {
		t.Fatalf("currency mismatch: %+v", ds)
	}
}

func TestTracfinCatalogSourceFingerprints(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Sources) != 23 {
		t.Fatalf("want original XSD + 22 raw CSV, got %d", len(c.Sources))
	}
	if c.Sources[0].SHA256 != "393e07591f988b2d58e2c889dd57a3751e428489e25728422816693898704f12" {
		t.Fatal("original supplied XSD changed")
	}
	for _, s := range c.Sources {
		if strings.HasSuffix(s.Path, ".csv") && (s.SourceURL == "" || s.RetrievedOn != "2026-10-21") {
			t.Fatalf("missing archived provenance: %s", s.Path)
		}
	}
	data, err := profileFiles.ReadFile("profile/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		InventorySource Source `json:"inventory_source"`
	}
	if err = json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	viewer, err := os.ReadFile(m.InventorySource.Path)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(viewer)
	if hex.EncodeToString(h[:]) != m.InventorySource.SHA256 {
		t.Fatal("viewer parity evidence changed")
	}
}

func TestTracfinCatalogScalarAttributeOwner(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if !c.CompatibleParent("/ds/personnes/pp/etatCivil", "/ds/personnes/pp/etatCivil/paysNaissance/@type") {
		t.Fatal("scalar attribute must be editable on its enclosing block")
	}
	if c.CompatibleParent("/ds/personnes/pm", "/ds/personnes/pp/etatCivil/paysNaissance/@type") {
		t.Fatal("scalar attribute crossed block boundary")
	}
}
