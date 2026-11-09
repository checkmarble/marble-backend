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
	"github.com/cockroachdb/errors"
)

// Validation is for the fixed embedded profile; it is not XSD certification.
func newValidation(c Catalog) models.TracfinValidation {
	v := models.TracfinValidation{ProfileVersion: c.Version, Layers: map[string]models.TracfinValidationLayer{}}
	for _, layer := range []string{"internal_structure", "references", "business", "xml_syntax"} {
		v.Layers[layer] = models.TracfinValidationLayer{Status: "passed"}
	}
	v.Layers["official_xsd"] = models.TracfinValidationLayer{Status: "unavailable", Reason: "Official XSD is evaluated separately on serialized XML; internal checks are not XSD certification"}

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
	return models.TracfinFieldRef{BlockID: b.ID, FieldID: f.ID, ValueID: val.ID, ParentValueID: val.ParentValueID}
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
	if f.Name == "idPieceJointe" || f.Name == "idPieceJointeDonneesConnexion" {
		if target.CatalogID != "/ds/pieceJointe/piece" {
			return nil, "Piece reference must identify an explicitly selected document piece"
		}
	} else if f.Name == "refFinanciereEmission" || f.Name == "refFinanciereReception" || f.Name == "refFinanciere" {
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
	if !c.CompatibleProfile(draft.Configuration.ProfileVersion, draft.Configuration.ProfileHash) {
		addValidation(&v, validationDiagnostic("INCOMPATIBLE_PROFILE", "internal_structure", "Draft profile does not match the embedded version/hash", models.TracfinFieldRef{}))
	}
	if draft.Document.Root.CatalogID != "/ds" {
		addValidation(&v, validationDiagnostic("UNKNOWN_DOCUMENT_BLOCK", "internal_structure", "Document root must be /ds", models.TracfinFieldRef{BlockID: draft.Document.Root.ID, FieldID: draft.Document.Root.CatalogID}))
	}
	var visit func(models.TracfinBlock)
	visit = func(b models.TracfinBlock) {
		if f, ok := c.Fields[b.CatalogID]; !ok || f.Kind != "block" || f.Coverage == "excluded" {
			addValidation(&v, validationDiagnostic("UNKNOWN_DOCUMENT_BLOCK", "internal_structure", "Unknown or excluded document block", models.TracfinFieldRef{BlockID: b.ID, FieldID: b.CatalogID}))
			return
		}
		for id := range b.Fields {
			if f, ok := c.Fields[id]; !ok || f.Kind != "field" || !c.CompatibleParent(b.CatalogID, id) {
				addValidation(&v, validationDiagnostic("UNKNOWN_DOCUMENT_FIELD", "internal_structure", "Unknown or incompatible document field", models.TracfinFieldRef{BlockID: b.ID, FieldID: id}))
			}
		}
		for _, child := range b.Children {
			if f, ok := c.Fields[child.CatalogID]; !ok || f.Kind != "block" || f.Parent != b.CatalogID || f.Coverage == "excluded" {
				addValidation(&v, validationDiagnostic("UNKNOWN_DOCUMENT_BLOCK", "internal_structure", "Unknown or incompatible document block", models.TracfinFieldRef{BlockID: child.ID, FieldID: child.CatalogID}))
			}
		}
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
			if f.Attribute && c.Fields[f.Parent].Kind == "field" {
				if !activeField(b, c.Fields[f.Parent], c) {
					continue
				}
				for _, parent := range b.Fields[f.Parent] {
					if _, present := valueText(parent); !present {
						continue
					}
					count := 0
					for _, attr := range b.Fields[id] {
						if attr.ParentValueID != parent.ID {
							continue
						}
						if _, present := valueText(attr); present {
							count++
						}
						validateValue(&v, b, f, attr, c)
					}
					ref := models.TracfinFieldRef{BlockID: b.ID, FieldID: f.ID, ParentValueID: parent.ID}
					min, _ := strconv.Atoi(f.Min)
					max, err := strconv.Atoi(f.Max)
					if count < min && f.Coverage != "calculated" {
						addValidation(&v, validationDiagnostic("MISSING_REQUIRED_VALUE", "internal_structure", "Required scalar attribute is absent", ref))
					}
					if err == nil && count > max {
						addValidation(&v, validationDiagnostic("CARDINALITY_EXCEEDED", "internal_structure", fmt.Sprintf("Maximum %d attributes per scalar occurrence", max), ref))
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
			min, _ := strconv.Atoi(f.Min)
			max, err := strconv.Atoi(f.Max)
			if count < min && f.Coverage != "calculated" {
				addValidation(&v, validationDiagnostic("MISSING_REQUIRED_VALUE", "internal_structure", "Required profile value is absent", fieldRef(b, f, models.TracfinValue{})))
			}
			if err == nil && count > max {
				addValidation(&v, validationDiagnostic("CARDINALITY_EXCEEDED", "internal_structure", fmt.Sprintf("Maximum %d occurrences", max), fieldRef(b, f, models.TracfinValue{})))
			}
		}
		for _, choice := range c.Choices {
			if choice.Parent != b.CatalogID {
				continue
			}
			count := 0
			for id, values := range b.Fields {
				if c.Fields[id].Choice == choice.ID && (b.ActiveBranches[choice.ID] == "" || activeField(b, c.Fields[id], c)) {
					for _, val := range values {
						if _, present := valueText(val); present || val.Reference != nil {
							count++
						}
					}
				}
			}
			for _, child := range b.Children {
				if c.Fields[child.CatalogID].Choice == choice.ID && (b.ActiveBranches[choice.ID] == "" || activeField(b, c.Fields[child.CatalogID], c)) {
					count++
				}
			}
			min, _ := strconv.Atoi(choice.Min)
			max, err := strconv.Atoi(choice.Max)
			if count < min {
				addValidation(&v, validationDiagnostic("MISSING_REQUIRED_CHOICE", "internal_structure", "Required choice is absent", models.TracfinFieldRef{BlockID: b.ID, FieldID: choice.ID}))
			}
			if err == nil && count > max && b.ActiveBranches[choice.ID] == "" {
				addValidation(&v, validationDiagnostic("UNRESOLVED_ALTERNATIVE_NOT_EXPORTED", "business", "Competing alternatives require an explicit selected branch", models.TracfinFieldRef{BlockID: b.ID, FieldID: choice.ID}))
			}
		}
		validateBusinessRules(&v, b)
		for _, child := range b.Children {
			if activeField(b, c.Fields[child.CatalogID], c) {
				visit(child)
			}
		}
	}
	visit(draft.Document.Root)
	validateConditionalRules(&v, draft.Document.Root, c)
	validateRuleCoverage(&v, draft.Document.Root, c)
	return v
}

// validateBusinessRules contains only the explicitly implemented source rules.
// Both the draft and independent XML reader supply their own represented values.
func validateBusinessRules(report *models.TracfinValidation, b models.TracfinBlock) {
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
				addValidation(report, validationDiagnostic("IDENTITY_PIECE_DETAILS_REQUIRED", "business", "Identity document requires identity details", models.TracfinFieldRef{BlockID: b.ID, FieldID: b.CatalogID + "/infoPieceIdentite"}))
			}
		}
	}
	if strings.HasSuffix(b.CatalogID, "/referencesFinancieres") && len(b.Children) > 20 {
		addValidation(report, validationDiagnostic("FINANCIAL_REFERENCE_LIMIT", "business", "A person may have at most 20 financial references combined", models.TracfinFieldRef{BlockID: b.ID, FieldID: b.CatalogID}))
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
					addValidation(report, validationDiagnostic("OTHER_OPERATION_DESCRIPTION_REQUIRED", "business", "Other operation requires its description", models.TracfinFieldRef{BlockID: b.ID, FieldID: b.CatalogID + "/autretypeOperation"}))
				}
			}
		}
	}
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
	var forbidden *XMLFieldError
	if errors.As(checkXMLText(text, ref), &forbidden) {
		addValidation(report, validationDiagnostic("XML_FORBIDDEN_CHARACTER", "internal_structure", forbidden.Message+"; export is refused until the value is corrected", ref))
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
			addValidation(report, validationDiagnostic(code, "business", "Code is absent from the available field reference catalogues; use a value from "+strings.Join(f.Catalogs, " or "), ref))
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
			add("INVALID_INTEGER", "Expected "+ExpectedFormat(f))
		}
	case "xs:decimal":
		if !decimalLexeme.MatchString(text) {
			add("INVALID_DECIMAL", "Expected "+ExpectedFormat(f))
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
				add("VALUE_TOO_SHORT", fmt.Sprintf("Value is shorter than the profile minimum of %d characters", n))
			}
		case "maxLength":
			if utf8.RuneCountInString(text) > n {
				add("VALUE_TOO_LONG", fmt.Sprintf("Value exceeds the profile maximum of %d characters", n))
			}
		case "enumeration":
			enumeration = append(enumeration, facet.Value)
		case "pattern":
			re, err := regexp.Compile("^(?:" + facet.Value + ")$")
			if err != nil {
				add("PATTERN_UNAVAILABLE", "Internal pattern checker cannot execute this source pattern")
			} else if !re.MatchString(text) {
				add("PATTERN_MISMATCH", "Value does not match the profile pattern; expected "+ExpectedFormat(f))
			}
		case "totalDigits":
			if decimalLexeme.MatchString(text) {
				parts := strings.Split(strings.TrimLeft(text, "+-"), ".")
				digits := strings.TrimLeft(parts[0], "0")
				if len(parts) > 1 {
					digits += strings.TrimRight(parts[1], "0")
				}
				digits = strings.TrimLeft(digits, "0")
				if len(digits) == 0 {
					digits = "0"
				}
				if len(digits) > n {
					add("TOTAL_DIGITS_EXCEEDED", fmt.Sprintf("Decimal exceeds the profile limit of %d significant digits", n))
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
			add("INVALID_ENUMERATION", "Value is not in the profile enumeration; allowed: "+strings.Join(enumeration, ", "))
		}
	}
}
