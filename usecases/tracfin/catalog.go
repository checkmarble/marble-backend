// Package tracfin implements the fixed financier TRACFIN profile. The embedded
// manifest is an inventory of this one supplied schema, not a schema interpreter.
package tracfin

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"golang.org/x/text/encoding/charmap"
)

//go:embed profile/manifest.json profile/coverage.json profile/sources/* profile/sources/referentiels/*
var profileFiles embed.FS

type Constraint struct {
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	SourceLine int    `json:"source_line"`
}

type Field struct {
	ID                string       `json:"id"`
	Parent            string       `json:"parent"`
	Name              string       `json:"name"`
	XSDType           string       `json:"xsd_type"`
	SourceLine        int          `json:"source_line"`
	Order             int          `json:"order"`
	Kind              string       `json:"kind"`
	Min               string       `json:"min"`
	Max               string       `json:"max"`
	Attribute         bool         `json:"attribute"`
	Coverage          string       `json:"coverage"`
	Model             string       `json:"model"`
	Reason            string       `json:"reason,omitempty"`
	ValueKind         string       `json:"value_kind"`
	Base              string       `json:"base"`
	Documentation     string       `json:"documentation"`
	TypeDocumentation string       `json:"type_documentation"`
	Choice            string       `json:"choice,omitempty"`
	Constraints       []Constraint `json:"constraints"`
	Catalogs          []string     `json:"catalogs"`
	Currency          string       `json:"currency,omitempty"`
	CurrencyStatus    string       `json:"currency_status,omitempty"`
	CurrencyReason    string       `json:"currency_reason,omitempty"`
}

type Source struct {
	SourceURL       string `json:"source_url,omitempty"`
	RetrievedOn     string `json:"retrieved_on,omitempty"`
	ProfessionScope string `json:"profession_scope,omitempty"`
	Compatibility   string `json:"compatibility,omitempty"`
	Path            string `json:"path"`
	Origin          string `json:"origin"`
	SHA256          string `json:"sha256"`
	Encoding        string `json:"encoding"`
}

type SourceRule struct {
	ID     string   `json:"id"`
	Source string   `json:"source"`
	Line   int      `json:"line,omitempty"`
	Kind   string   `json:"kind"`
	Text   string   `json:"text"`
	Fields []string `json:"fields"`
	Status string   `json:"status"`
	Reason string   `json:"reason"`
}

type Choice struct {
	ID         string `json:"id"`
	Parent     string `json:"parent"`
	Min        string `json:"min"`
	Max        string `json:"max"`
	SourceLine int    `json:"source_line"`
}

type Code struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Catalog struct {
	Version             string              `json:"version"`
	Hash                string              `json:"hash"`
	SourceVersion       string              `json:"source_version"`
	Sources             []Source            `json:"sources"`
	Fields              map[string]Field    `json:"fields"`
	OrderedFields       []string            `json:"ordered_fields"`
	Choices             []Choice            `json:"choices"`
	Rules               []SourceRule        `json:"rules"`
	Codes               map[string][]Code   `json:"codes"`
	CatalogBindings     map[string]string   `json:"catalog_bindings"`
	IngestedEntityTypes []string            `json:"ingested_entity_types"`
	ExcludedCodes       map[string][]string `json:"excluded_codes"`
}

// LoadCatalog verifies every embedded source fingerprint before serving the
// profile. No generated example values are used as a code authority.
func LoadCatalog() (Catalog, error) {
	manifest, err := profileFiles.ReadFile("profile/manifest.json")
	if err != nil {
		return Catalog{}, err
	}
	coverage, err := profileFiles.ReadFile("profile/coverage.json")
	if err != nil {
		return Catalog{}, err
	}
	var m struct {
		Version             string              `json:"version"`
		SourceVersion       string              `json:"source_version"`
		Sources             []Source            `json:"sources"`
		Fields              []Field             `json:"fields"`
		Choices             []Choice            `json:"choices"`
		CatalogBindings     map[string]string   `json:"catalog_bindings"`
		IngestedEntityTypes []string            `json:"ingested_entity_types"`
		ExcludedCodes       map[string][]string `json:"excluded_codes"`
	}
	if err = json.Unmarshal(manifest, &m); err != nil {
		return Catalog{}, err
	}
	var rules struct {
		Rules []SourceRule `json:"rules"`
	}
	if err = json.Unmarshal(coverage, &rules); err != nil {
		return Catalog{}, err
	}
	h := sha256.New()
	h.Write(manifest)
	h.Write(coverage)
	c := Catalog{Version: m.Version, SourceVersion: m.SourceVersion, Hash: hex.EncodeToString(h.Sum(nil)), Sources: m.Sources, Fields: make(map[string]Field), Choices: m.Choices, Rules: rules.Rules, Codes: make(map[string][]Code), CatalogBindings: m.CatalogBindings, IngestedEntityTypes: m.IngestedEntityTypes, ExcludedCodes: m.ExcludedCodes}
	for _, f := range m.Fields {
		if _, exists := c.Fields[f.ID]; exists {
			return Catalog{}, errors.Errorf("duplicate profile path %s", f.ID)
		}
		c.Fields[f.ID] = f
		c.OrderedFields = append(c.OrderedFields, f.ID)
	}
	for _, src := range c.Sources {
		data, err := profileFiles.ReadFile("profile/" + src.Path)
		if err != nil {
			return Catalog{}, err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != src.SHA256 {
			return Catalog{}, errors.Errorf("profile source hash mismatch: %s", src.Path)
		}
		if !strings.HasSuffix(src.Path, ".csv") {
			continue
		}
		decoded, err := charmap.Windows1252.NewDecoder().Bytes(data)
		if err != nil {
			return Catalog{}, err
		}
		rows := csv.NewReader(bytes.NewReader(decoded))
		rows.Comma = ';'
		header, err := rows.Read()
		if err != nil || len(header) != 2 || header[0] != "code" || header[1] != "label" {
			return Catalog{}, errors.Errorf("invalid catalog header: %s", src.Path)
		}
		name := strings.TrimPrefix(src.Path, "sources/referentiels/")
		c.Codes[name] = []Code{}
		seen := map[string]bool{}
		for {
			row, err := rows.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return Catalog{}, errors.Wrapf(err, "read catalog %s", name)
			}
			if len(row) != 2 || row[0] == "" || seen[row[0]] {
				return Catalog{}, errors.Errorf("invalid or duplicate code in %s", name)
			}
			seen[row[0]] = true
			c.Codes[name] = append(c.Codes[name], Code{Value: row[0], Label: row[1]})
		}
	}
	return c, nil
}

// CodeStatus distinguishes a missing authority from an unknown code. TA17 is
// retained in its unchanged source but explicitly unavailable for selection.
func (c Catalog) CodeStatus(name, value string) string {
	codes, ok := c.Codes[name]
	if !ok {
		return "unavailable"
	}
	for _, v := range c.ExcludedCodes[name] {
		if value == v {
			return "excluded"
		}
	}
	for _, code := range codes {
		if code.Value == value {
			return "known"
		}
	}
	return "unknown"
}

// CompatibleParent checks the enclosing document block. Attributes on simple
// content (e.g. pays/@type) live in that block's Fields with their full catalog
// ID; Field.Parent still preserves the exact XML scalar parent for serialization.
func (c Catalog) CompatibleParent(parent, child string) bool {
	f, ok := c.Fields[child]
	if !ok || f.Coverage == "excluded" {
		return false
	}
	owner := f.Parent
	for {
		p, exists := c.Fields[owner]
		if !exists || p.Kind == "block" {
			break
		}
		owner = p.Parent
	}
	return owner == parent
}

func ValidateConfiguration(config models.TracfinConfiguration, catalog Catalog) []models.TracfinDiagnostic {
	var diagnostics []models.TracfinDiagnostic
	add := func(code, message, field string) {
		diagnostics = append(diagnostics, models.TracfinDiagnostic{Code: code, Layer: "configuration", Severity: "error", Message: message, Field: &models.TracfinFieldRef{FieldID: field}})
	}
	if config.ProfileVersion != "" && config.ProfileVersion != catalog.Version || config.ProfileHash != "" && config.ProfileHash != catalog.Hash {
		add("INCOMPATIBLE_PROFILE", "Configuration profile version/hash does not match embedded profile", "")
	}
	ids := map[string]bool{}
	for _, mapping := range config.Mappings {
		if mapping.ID == "" || ids[mapping.ID] {
			add("INVALID_MAPPING_ID", "Mapping ID must be nonempty and unique", mapping.TargetField)
		}
		ids[mapping.ID] = true
		field, ok := catalog.Fields[mapping.TargetField]
		if !ok || field.Kind != "field" || field.Coverage != "editable" {
			add("INVALID_MAPPING_TARGET", "Target must be an editable TRACFIN field", mapping.TargetField)
			continue
		}
		switch mapping.Source.Kind {
		case "ingested":
			if mapping.Source.Table == "" || mapping.Source.Field == "" {
				add("INVALID_MAPPING_SOURCE", "Ingested source requires table and field", field.ID)
			}
			if mapping.Source.EntityType != "Person" && mapping.Source.EntityType != "Company" {
				add("INVALID_ENTITY_TYPE", "Only Person/Company ingested roots are supported", field.ID)
			}
		case "decision_snapshot":
			if mapping.Source.HistoricalType == "" || mapping.Source.Field == "" {
				add("INVALID_MAPPING_SOURCE", "Snapshot source requires historical type and field", field.ID)
			}
		case "settings":
			if mapping.Source.Field == "" {
				add("INVALID_MAPPING_SOURCE", "Settings source requires a field", field.ID)
			}
		default:
			add("INVALID_MAPPING_SOURCE", "Unknown mapping source kind", field.ID)
		}

		if mapping.Source.UnitOperation && (mapping.Source.Kind != "decision_snapshot" || !strings.HasSuffix(field.ID, "/operation/nombreOperations")) {
			add("INVALID_UNIT_OPERATION", "Unit operation requires an explicit decision snapshot mapped to principal operation count", field.ID)
		}
		if mapping.Discriminator != nil {
			if mapping.Discriminator.Field == "" || len(mapping.Discriminator.Values) == 0 {
				add("INVALID_DISCRIMINATOR", "Discriminator requires a field and explicit accepted source values", field.ID)
			}
			seen := map[string]bool{}
			for _, value := range mapping.Discriminator.Values {
				if value == "" || seen[value] {
					add("INVALID_DISCRIMINATOR", "Discriminator source values must be nonempty and unique", field.ID)
				}
				seen[value] = true
			}
		}
		if len(mapping.Source.Path) > 20 || (mapping.Source.Kind != "ingested" && len(mapping.Source.Path) > 0) {
			add("INVALID_MAPPING_PATH", "Only ingested paths up to 20 jumps are supported", field.ID)
		}
		parent := "root"
		steps := map[string]bool{"root": true}
		for _, step := range mapping.Source.Path {
			if step.ID == "" || steps[step.ID] || step.ParentID != parent || step.Table == "" || step.Link == "" || step.RecordID != "" || (step.Direction != "forward" && step.Direction != "inverse") {
				add("INVALID_MAPPING_PATH", "Path must use stable unique step IDs, configured named edges and no selected record IDs", field.ID)
			}
			steps[step.ID] = true
			parent = step.ID
		}
		values := map[string]string{}
		for _, pair := range mapping.Codes {
			if strings.TrimSpace(pair.Source) == "" {
				add("MISSING_SOURCE_CODE", "Source code must be entered explicitly", field.ID)
			}
			if prev, ok := values[pair.Source]; ok && prev != pair.Target {
				add("CONTRADICTORY_CODE_MAPPING", "Same source value maps to different target codes", field.ID)
			}
			values[pair.Source] = pair.Target
			known, unavailable, excluded := false, false, false
			for _, name := range field.Catalogs {
				switch catalog.CodeStatus(name, pair.Target) {
				case "known":
					known = true
				case "unavailable":
					unavailable = true
				case "excluded":
					excluded = true
				}
			}
			// Enumerated XSD values are also a source authority when no CSV is referenced.
			if len(field.Catalogs) == 0 {
				for _, constraint := range field.Constraints {
					if constraint.Kind == "enumeration" && constraint.Value == pair.Target {
						known = true
					}
				}
			}
			if !known {
				code := "UNKNOWN_CATALOG_CODE"
				if unavailable {
					code = "CATALOG_UNAVAILABLE"
				}
				if excluded {
					code = "EXCLUDED_CATALOG_CODE"
				}
				add(code, "Target code must be known in the field's source catalog", field.ID)
			}
		}
		if mapping.Unit != "" && mapping.Unit != "major" && mapping.Unit != "minor" {
			add("INVALID_AMOUNT_UNIT", "Amount unit must be major or minor", field.ID)
		}
		if field.ValueKind == "decimal" || mapping.TargetCurrency != "" || mapping.Unit != "" || mapping.Currency.Field != "" || mapping.Currency.Constant != "" {
			if field.ValueKind != "decimal" {
				add("INVALID_CURRENCY_TARGET", "Currency evidence requires a decimal target", field.ID)
			}
			if mapping.TargetCurrency == "" {
				add("MISSING_TARGET_CURRENCY", "Monetary mapping requires an explicit target currency", field.ID)
			}
			if field.Currency != "" && mapping.TargetCurrency != field.Currency {
				add("INCOMPATIBLE_TARGET_CURRENCY", "Configured target currency differs from proven profile currency", field.ID)
			}
			if (mapping.Currency.Field == "") == (mapping.Currency.Constant == "") {
				add("INVALID_CURRENCY_EVIDENCE", "Provide exactly one source currency field or constant", field.ID)
			}
			if mapping.Currency.Constant != "" && mapping.Currency.Constant != mapping.TargetCurrency {
				add("INCOMPATIBLE_SOURCE_CURRENCY", "Currency conversion is not supported", field.ID)
			}
			if mapping.TargetCurrency != "" && catalog.CodeStatus("devises.csv", mapping.TargetCurrency) != "known" {
				add("UNKNOWN_CURRENCY", "Target currency must be known", field.ID)
			}
		}
	}
	return diagnostics
}
