package tracfin

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// AIFieldSpec is the compact, human-readable contract of one catalogue entry, as shown to the
// model. It replaces raw XSD facets (regexes, source lines) with the expected value format.
type AIFieldSpec struct {
	Kind         string   `json:"kind"`
	Coverage     string   `json:"coverage"`
	OwnerBlock   string   `json:"owner_block,omitempty"`
	ParentScalar string   `json:"parent_scalar,omitempty"`
	Occurs       string   `json:"occurs"`
	ValueKind    string   `json:"value_kind,omitempty"`
	Format       string   `json:"format,omitempty"`
	Allowed      []string `json:"allowed,omitempty"`
	CodeLists    []string `json:"code_lists,omitempty"`
	Choice       string   `json:"choice,omitempty"`
	Currency     string   `json:"currency,omitempty"`
	Help         string   `json:"help,omitempty"`
}

const aiSpecHelpLimit = 400

func (c Catalog) AISpec(id string) AIFieldSpec {
	f := c.Fields[id]
	max := f.Max
	if max == "" {
		max = "1"
	}
	occurs := f.Min + ".." + max
	if f.Min == max {
		occurs = max
	}
	spec := AIFieldSpec{Kind: f.Kind, Coverage: f.Coverage, Occurs: occurs, Choice: f.Choice, Currency: f.Currency, Help: aiSpecHelp(f)}
	if f.Kind != "field" {
		return spec
	}
	spec.OwnerBlock = c.OwnerBlock(id)
	if parent, ok := c.Fields[f.Parent]; ok && parent.Kind == "field" {
		spec.ParentScalar = f.Parent
	}
	spec.ValueKind = f.ValueKind
	spec.Format = ExpectedFormat(f)
	spec.CodeLists = f.Catalogs
	for _, facet := range f.Constraints {
		if facet.Kind == "enumeration" {
			spec.Allowed = append(spec.Allowed, facet.Value)
		}
	}
	return spec
}

func aiSpecHelp(f Field) string {
	text := strings.Join(strings.Fields(strings.TrimSpace(f.Documentation+" "+f.TypeDocumentation)), " ")
	if utf8.RuneCountInString(text) > aiSpecHelpLimit {
		text = string([]rune(text)[:aiSpecHelpLimit]) + "…"
	}
	return text
}

// friendlyPatterns names the source patterns in terms a model or an analyst understands.
var friendlyPatterns = map[string]string{
	`\d{2}/\d{2}/\d{4}`: "date DD/MM/YYYY (for example 14/07/1985)",
	`\d{2}:\d{2}`:       "time HH:MM (24h)",
	`[0-9]{5}`:          "exactly 5 digits",
	`[a-zA-Z0-9 ]*`:     "only unaccented letters, digits and spaces",
	`^[a-zA-Z]{2}[0-9]{2}[a-zA-Z0-9]{10,30}$`:      "IBAN without spaces (2 letters, 2 digits, then 10 to 30 letters or digits)",
	`^[+]*[(]{0,1}[0-9]{1,4}[)]{0,1}[-\s\./0-9]*$`: "phone number: optional +, country code, digits, spaces, dots or dashes",
	`^[\w-\._]+@([\w-]+\.)+[\w-]{2,4}$`:            "email address",
}

// ExpectedFormat describes the value a field accepts, for prompts and validation messages.
func ExpectedFormat(f Field) string {
	var parts []string
	switch {
	case f.ValueKind == "reference":
		parts = append(parts, "reference to a declared block: kind reference with reference_key")
	case f.ValueKind == "boolean" || f.Base == "xs:boolean":
		parts = append(parts, "boolean")
	case f.ValueKind == "decimal" || f.Base == "xs:decimal":
		parts = append(parts, "decimal number without exponent")
	case f.Base == "xs:positiveInteger":
		parts = append(parts, "positive integer")
	case f.Base == "xs:integer":
		parts = append(parts, "integer")
	default:
		parts = append(parts, "text")
	}
	minLength, maxLength := "", ""
	for _, facet := range f.Constraints {
		switch facet.Kind {
		case "minLength":
			minLength = facet.Value
		case "maxLength":
			maxLength = facet.Value
		case "totalDigits":
			parts = append(parts, "at most "+facet.Value+" significant digits")
		case "pattern":
			if friendly, ok := friendlyPatterns[facet.Value]; ok {
				parts = append(parts, friendly)
			} else {
				parts = append(parts, "must match the regular expression "+facet.Value)
			}
		case "enumeration":
			if !strings.Contains(strings.Join(parts, ";"), "one of the allowed values") {
				parts = append(parts, "one of the allowed values")
			}
		}
	}
	switch {
	case minLength != "" && maxLength != "":
		parts = append(parts, fmt.Sprintf("%s to %s characters", minLength, maxLength))
	case maxLength != "":
		parts = append(parts, "at most "+maxLength+" characters")
	case minLength != "":
		parts = append(parts, "at least "+minLength+" characters")
	}
	if len(f.Catalogs) > 0 {
		parts = append(parts, "a code from "+strings.Join(f.Catalogs, " or "))
	}
	return strings.Join(parts, "; ")
}
