package tracfin

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/checkmarble/marble-backend/models"
)

// Validation is for the fixed embedded profile; it is not XSD certification.
func newValidation(c Catalog) models.TracfinValidation {
	v := models.TracfinValidation{ProfileVersion: c.Version, Layers: map[string]models.TracfinValidationLayer{}}
	for _, layer := range []string{"internal_structure", "references", "business", "xml_syntax"} {
		v.Layers[layer] = models.TracfinValidationLayer{Status: "passed"}
	}
	v.Layers["official_xsd"] = models.TracfinValidationLayer{Status: "unavailable", Reason: "No compatible official XSD engine is integrated; internal checks are not certification"}
	for _, rule := range c.Rules {
		if rule.Status == "unavailable" && rule.Kind != "official_xsd" {
			for _, layer := range []string{"internal_structure", "references", "business"} {
				v.Layers[layer] = models.TracfinValidationLayer{Status: "incomplete", Reason: "Some registered source rules have not been implemented and tested; see coverage.json"}
			}
			break
		}
	}
	return v
}

func addValidation(v *models.TracfinValidation, d models.TracfinDiagnostic) {
	v.Diagnostics = append(v.Diagnostics, d)
	if d.Severity == "error" {
		l := v.Layers[d.Layer]
		l.Status = "failed"
		v.Layers[d.Layer] = l
	}
}

func validationDiagnostic(code, layer, message string, ref models.TracfinFieldRef) models.TracfinDiagnostic {
	return models.TracfinDiagnostic{Code: code, Layer: layer, Severity: "error", Message: message, Field: &ref}
}

func valueText(v models.TracfinValue) (string, bool) {
	if v.Kind == "cleared" || v.Origin != nil && v.Origin.Resolution == "conflict" {
		return "", false
	}
	switch v.Kind {
	case "text":
		if v.Text != nil {
			return *v.Text, true
		}
	case "decimal":
		if v.Decimal != nil {
			return string(*v.Decimal), true
		}
	case "boolean":
		if v.Boolean != nil {
			return strconv.FormatBool(*v.Boolean), true
		}
	}
	return "", false
}

func fieldRef(b models.TracfinBlock, f Field, val models.TracfinValue) models.TracfinFieldRef {
	return models.TracfinFieldRef{BlockID: b.ID, FieldID: f.ID, ValueID: val.ID}
}

func activeField(b models.TracfinBlock, f Field, c Catalog) bool {
	if f.Choice == "" {
		return true
	}
	if selected := b.ActiveBranches[f.Choice]; selected != "" {
		return selected == f.ID
	}
	// A single retained alternative is unambiguous. Competing alternatives
	// without a selected branch must never acquire an implicit first winner.
	count := 0
	for id, values := range b.Fields {
		if c.Fields[id].Choice == f.Choice {
			for _, value := range values {
				if value.Kind != "cleared" {
					count++
					break
				}
			}
		}
	}
	for _, child := range b.Children {
		if c.Fields[child.CatalogID].Choice == f.Choice {
			count++
		}
	}
	return count <= 1
}

func hasBlockData(b models.TracfinBlock) bool {
	if b.Attachment != nil {
		return true
	}
	for _, values := range b.Fields {
		for _, value := range values {
			if value.Kind != "cleared" {
				return true
			}
		}
	}
	for _, child := range b.Children {
		if hasBlockData(child) {
			return true
		}
	}
	return false
}

func referenceTarget(root models.TracfinBlock, owner models.TracfinBlock, f Field, val models.TracfinValue) (*models.TracfinBlock, string) {
	if val.Reference == nil {
		return nil, "Reference value is absent"
	}
	target := findBlock(&root, val.Reference.BlockID)
	if target == nil {
		return nil, "Referenced block is absent"
	}
	if f.Name == "refFinanciereEmission" || f.Name == "refFinanciereReception" || f.Name == "refFinanciere" {
		personField := "emetteurTypeEmetteur"
		if f.Name == "refFinanciereReception" {
			personField = "beneficiaireTypeBeneficiaire"
		}
		if f.Name == "refFinanciere" {
			personField = "identEmetteur"
		}
		person := ""
		for _, value := range owner.Fields[owner.CatalogID+"/"+personField] {
			if value.Reference != nil {
				person = value.Reference.BlockID
			}
		}
		if person == "" || val.Reference.PersonBlockID != "" && val.Reference.PersonBlockID != person {
			return nil, "Financial reference must identify its selected person context"
		}
		p := findBlock(&root, person)
		if p == nil || findBlock(p, target.ID) == nil || !strings.Contains(target.CatalogID, "/referencesFinancieres/referenceFinanciere") {
			return nil, "Financial reference does not belong to the selected person"
		}
	} else if target.CatalogID != "/ds/personnes/pp" && target.CatalogID != "/ds/personnes/pm" && target.CatalogID != "/ds/personnes/pnc" {
		return nil, "Reference target is not a declared person"
	}
	return target, ""
}

// ValidateDraft preserves the supplied document and reports missing or invalid values.
func ValidateDraft(draft models.TracfinDraft, c Catalog) models.TracfinValidation {
	v := newValidation(c)
	v.Layers["xml_syntax"] = models.TracfinValidationLayer{Status: "unavailable", Reason: "Draft validation does not read exported XML bytes"}
	if draft.Configuration.ProfileVersion != "" && draft.Configuration.ProfileVersion != c.Version || draft.Configuration.ProfileHash != "" && draft.Configuration.ProfileHash != c.Hash {
		addValidation(&v, validationDiagnostic("INCOMPATIBLE_PROFILE", "internal_structure", "Draft profile does not match the embedded version/hash", models.TracfinFieldRef{}))
	}
	var visit func(models.TracfinBlock)
	visit = func(b models.TracfinBlock) {
		for _, id := range c.OrderedFields {
			f := c.Fields[id]
			if f.Coverage == "excluded" || !c.CompatibleParent(b.CatalogID, id) {
				continue
			}
			if !activeField(b, f, c) {
				code, message := "INACTIVE_BRANCH_DATA_NOT_EXPORTED", "Inactive branch data is retained but omitted"
				if b.ActiveBranches[f.Choice] == "" {
					code, message = "UNRESOLVED_ALTERNATIVE_NOT_EXPORTED", "Competing alternatives require an explicit selected branch"
				}
				if f.Kind == "block" {
					for _, child := range b.Children {
						if child.CatalogID == id && hasBlockData(child) {
							addValidation(&v, validationDiagnostic(code, "business", message, models.TracfinFieldRef{BlockID: child.ID, FieldID: id}))
						}
					}
				} else {
					for _, value := range b.Fields[id] {
						if value.Kind != "cleared" {
							addValidation(&v, validationDiagnostic(code, "business", message, fieldRef(b, f, value)))
						}
					}
				}
				continue
			}
			count := 0
			if f.Kind == "block" {
				for _, child := range b.Children {
					if child.CatalogID == id {
						count++
					}
				}
			} else {
				for _, val := range b.Fields[id] {
					if _, present := valueText(val); present || val.Reference != nil {
						count++
					}
					if val.Reference != nil {
						if _, reason := referenceTarget(draft.Document.Root, b, f, val); reason != "" {
							addValidation(&v, validationDiagnostic("DANGLING_REFERENCE_NOT_EXPORTED", "references", reason, fieldRef(b, f, val)))
						}
					}
					validateValue(&v, b, f, val, c)
				}
			}
			// Scalar attributes are counted for each owning occurrence, not per block.
			if f.Attribute && c.Fields[f.Parent].Kind == "field" {
				continue
			}
			min, _ := strconv.Atoi(f.Min)
			max, err := strconv.Atoi(f.Max)
			if count < min && f.Coverage != "calculated" {
				addValidation(&v, validationDiagnostic("MISSING_REQUIRED_VALUE", "internal_structure", "Required profile value is absent", fieldRef(b, f, models.TracfinValue{})))
			}
			if err == nil && count > max {
				addValidation(&v, validationDiagnostic("CARDINALITY_EXCEEDED", "internal_structure", fmt.Sprintf("Maximum %d occurrences", max), fieldRef(b, f, models.TracfinValue{})))
			}
		}
		if b.CatalogID == "/ds/pieceJointe/piece" {
			for _, val := range b.Fields[b.CatalogID+"/type"] {
				text, _ := valueText(val)
				if text != "TA2" && text != "TA3" && text != "TA5" {
					continue
				}
				found := false
				for _, child := range b.Children {
					if child.CatalogID == b.CatalogID+"/infoPieceIdentite" {
						found = true
					}
				}
				if !found {
					addValidation(&v, validationDiagnostic("IDENTITY_PIECE_DETAILS_REQUIRED", "business", "Identity document requires identity details", models.TracfinFieldRef{BlockID: b.ID, FieldID: b.CatalogID + "/infoPieceIdentite"}))
				}
			}
		}
		if strings.HasSuffix(b.CatalogID, "/referencesFinancieres") && len(b.Children) > 20 {
			addValidation(&v, validationDiagnostic("FINANCIAL_REFERENCE_LIMIT", "business", "A person may have at most 20 financial references combined", models.TracfinFieldRef{BlockID: b.ID, FieldID: b.CatalogID}))
		}
		if b.CatalogID == "/ds/operations/operation" || b.CatalogID == "/ds/operations/operationEnCours" {
			for _, value := range b.Fields[b.CatalogID+"/typeOperation"] {
				code, _ := valueText(value)
				if code == "OT11" {
					found := false
					for _, other := range b.Fields[b.CatalogID+"/autretypeOperation"] {
						text, present := valueText(other)
						if present && text != "" {
							found = true
						}
					}
					if !found {
						addValidation(&v, validationDiagnostic("OTHER_OPERATION_DESCRIPTION_REQUIRED", "business", "Other operation requires its description", models.TracfinFieldRef{BlockID: b.ID, FieldID: b.CatalogID + "/autretypeOperation"}))
					}
				}
			}
		}
		for _, child := range b.Children {
			if activeField(b, c.Fields[child.CatalogID], c) {
				visit(child)
			}
		}
	}
	visit(draft.Document.Root)
	return v
}

func validateValue(report *models.TracfinValidation, b models.TracfinBlock, f Field, val models.TracfinValue, c Catalog) {
	ref := fieldRef(b, f, val)
	if val.Origin != nil && val.Origin.Resolution == "conflict" {
		addValidation(report, validationDiagnostic("UNRESOLVED_CONFLICT_NOT_EXPORTED", "business", "Unresolved conflicting value is omitted", ref))
		return
	}
	text, present := valueText(val)
	if f.ValueKind == "reference" && val.Kind != "cleared" && val.Kind != "reference" {
		addValidation(report, validationDiagnostic("REFERENCE_REQUIRED", "references", "Field requires an explicitly selected declared object; numeric free text is not an internal reference", ref))
	}
	if !present {
		return
	}
	validateScalar(report, f, text, ref)
	if f.XSDType == "t_date" {
		if _, err := time.Parse("02/01/2006", text); err != nil {
			addValidation(report, validationDiagnostic("INVALID_DATE", "business", "Expected a valid civil date in DD/MM/YYYY format", ref))
		}
	}
	otherCountry := false
	for _, attr := range b.Fields[f.ID+"/@type"] {
		if attr.ParentValueID == val.ID {
			s, _ := valueText(attr)
			otherCountry = s == "other"
		}
	}
	if !otherCountry {
		known, unavailable := false, false
		for _, name := range f.Catalogs {
			status := c.CodeStatus(name, text)
			known = known || status == "known"
			unavailable = unavailable || status == "unavailable"
		}
		if len(f.Catalogs) > 0 && !known {
			code := "UNKNOWN_CODE"
			if unavailable {
				code = "CODE_CATALOG_UNAVAILABLE"
			}
			addValidation(report, validationDiagnostic(code, "business", "Code is absent from the available field reference catalogues", ref))
		}
	}
}

var integerLexeme = regexp.MustCompile(`^[+-]?[0-9]+$`)
var decimalLexeme = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)$`)

func validateScalar(report *models.TracfinValidation, f Field, text string, ref models.TracfinFieldRef) {
	add := func(code, message string) {
		addValidation(report, validationDiagnostic(code, "business", message, ref))
	}
	switch f.Base {
	case "xs:integer", "xs:positiveInteger":
		n, ok := new(big.Int).SetString(text, 10)
		if !integerLexeme.MatchString(text) || !ok || f.Base == "xs:positiveInteger" && n.Sign() <= 0 {
			add("INVALID_INTEGER", "Expected a valid integer of the profile type")
		}
	case "xs:decimal":
		if !decimalLexeme.MatchString(text) {
			add("INVALID_DECIMAL", "Expected an exact decimal without exponent")
		}
	case "xs:boolean":
		if text != "true" && text != "false" && text != "1" && text != "0" {
			add("INVALID_BOOLEAN", "Expected true, false, 1 or 0")
		}
	}
	var enumeration []string
	for _, facet := range f.Constraints {
		n, _ := strconv.Atoi(facet.Value)
		switch facet.Kind {
		case "minLength":
			if utf8.RuneCountInString(text) < n {
				add("VALUE_TOO_SHORT", "Value is shorter than the profile minimum")
			}
		case "maxLength":
			if utf8.RuneCountInString(text) > n {
				add("VALUE_TOO_LONG", "Value exceeds the profile maximum length")
			}
		case "enumeration":
			enumeration = append(enumeration, facet.Value)
		case "pattern":
			re, err := regexp.Compile("^(?:" + facet.Value + ")$")
			if err != nil {
				add("PATTERN_UNAVAILABLE", "Internal pattern checker cannot execute this source pattern")
			} else if !re.MatchString(text) {
				add("PATTERN_MISMATCH", "Value does not match the profile pattern")
			}
		case "totalDigits":
			if decimalLexeme.MatchString(text) {
				parts := strings.Split(strings.TrimLeft(text, "+-"), ".")
				digits := strings.TrimLeft(parts[0], "0")
				if len(parts) > 1 {
					digits += strings.TrimRight(parts[1], "0")
				}
				if len(digits) == 0 {
					digits = "0"
				}
				if len(digits) > n {
					add("TOTAL_DIGITS_EXCEEDED", "Decimal exceeds the profile digit limit")
				}
			}
		}
	}
	if len(enumeration) > 0 {
		found := false
		for _, allowed := range enumeration {
			found = found || text == allowed
		}
		if !found {
			add("INVALID_ENUMERATION", "Value is not in the profile enumeration")
		}
	}
}
