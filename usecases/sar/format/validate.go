package format

import (
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/checkmarble/marble-backend/models"
)

// Rule is a Go implementation of a conditional constraint or XSD 1.1 assert.
// Context is a complex type name: Check runs on every node of that type.
// An empty context runs once on the root.
type Rule struct {
	Code    string `json:"code"`
	Context string `json:"context"`
	Message string `json:"message"`
	Source  string `json:"source,omitempty"` // XSD line, guide, FAQ...
	Check   func(n *Node, path string, doc *Node) []Finding
}

// Finding is a rule violation, relative to the checked node.
type Finding struct {
	Path    string // absolute path; empty means the checked node
	Message string // overrides the rule message when set
}

// Validate checks structure, cardinality, choices, simple types, code lists
// and rules. Issues are localized by stable path and carry the edit target
// of the offending value when known.
func Validate(s *Schema, root *Node) []models.SarIssue {
	s.init()
	v := validator{s: s}
	if root == nil || root.Name != s.RootElement {
		return []models.SarIssue{{Path: "/" + s.RootElement, Code: "STRUCT_ROOT", Message: "Missing or unexpected root element", Severity: "error", Layer: "structure"}}
	}
	v.complex(root, "/"+root.Name, s.RootType)
	for i := range v.issues {
		if v.issues[i].Edit == "" {
			v.issues[i].Edit = editAt(root, v.issues[i].Path)
		}
	}
	// Rules
	types := map[*Node]string{root: s.RootType}
	paths := map[*Node]string{}
	Walk(root, func(n *Node, path string, parent *Node) {
		paths[n] = path
		if parent == nil {
			return
		}
		if pt, ok := types[parent]; ok {
			if e, ok := s.Lookup(pt, n.Name); ok {
				if _, complex := s.Complex[e.Type]; complex {
					types[n] = e.Type
				}
			}
		}
	})
	for _, r := range s.Rules {
		var targets []*Node
		if r.Context == "" {
			targets = []*Node{root}
		} else {
			Walk(root, func(n *Node, _ string, _ *Node) {
				if types[n] == r.Context {
					targets = append(targets, n)
				}
			})
		}
		for _, n := range targets {
			for _, f := range r.Check(n, paths[n], root) {
				p := f.Path
				if p == "" {
					p = paths[n]
				}
				msg := r.Message
				if f.Message != "" {
					msg = f.Message
				}
				v.add(p, r.Code, msg, "rule", editAt(root, p))
			}
		}
	}
	return v.issues
}

// editAt returns the edit target of the node at path, or of its closest
// ancestor carrying one (an absent element is fixed through its owner).
func editAt(root *Node, path string) string {
	best, bestLen := "", -1
	Walk(root, func(n *Node, p string, _ *Node) {
		if n.Meta == nil || n.Meta.Edit == "" {
			return
		}
		if (p == path || strings.HasPrefix(path, p+"/")) && len(p) > bestLen {
			best, bestLen = n.Meta.Edit, len(p)
		}
	})
	return best
}

type validator struct {
	s      *Schema
	issues []models.SarIssue
}

func (v *validator) add(path, code, msg, layer, edit string) {
	v.issues = append(v.issues, models.SarIssue{Path: path, Code: code, Message: msg, Severity: "error", Layer: layer, Edit: edit})
}

func edit(n *Node) string {
	if n != nil && n.Meta != nil {
		return n.Meta.Edit
	}
	return ""
}

func (v *validator) complex(n *Node, path, typeName string) {
	t := v.s.Complex[typeName]
	if t == nil {
		v.add(path, "STRUCT_UNKNOWN_TYPE", "Unknown type "+typeName, "structure", "")
		return
	}
	if t.Text != "" {
		if n.Text == nil {
			v.add(path, "CARD_MIN", "Value is required", "structure", edit(n))
		} else {
			v.simple(n, path, t.Text, "")
		}
	} else if n.Text != nil && strings.TrimSpace(*n.Text) != "" {
		v.add(path, "STRUCT_TEXT", "Element cannot carry a value", "structure", edit(n))
	}
	counts := map[string]int{}
	present := map[string]bool{}
	for _, c := range n.Children {
		counts[c.Name]++
		e, ok := v.s.Lookup(typeName, c.Name)
		cp := ChildPath(path, c, counts[c.Name])
		if !ok || e.Attribute != c.Attribute {
			v.add(cp, "STRUCT_UNKNOWN_ELEMENT", fmt.Sprintf("%s is not part of %s", c.Name, typeName), "structure", edit(c))
			continue
		}
		if e.Max == 0 {
			v.add(cp, "STRUCT_NOT_ALLOWED", fmt.Sprintf("%s is not accepted by this profile", c.Name), "structure", edit(c))
			continue
		}
		present[c.Name] = true
		if _, isComplex := v.s.Complex[e.Type]; isComplex {
			v.complex(c, cp, e.Type)
		} else if c.Text == nil {
			v.add(cp, "CARD_MIN", "Value is required", "structure", edit(c))
		} else {
			v.simple(c, cp, e.Type, e.Fixed)
		}
	}
	for _, ch := range t.Choices {
		n := 0
		for _, lead := range ch.Leads {
			if present[lead] {
				n++
			}
		}
		if n > 1 {
			v.add(path, "CHOICE_MULTIPLE", "Only one of "+strings.Join(ch.Leads, ", ")+" is allowed", "structure", "")
		}
		if n == 0 && ch.Min > 0 {
			allowed := []string{}
			for _, lead := range ch.Leads {
				if e, ok := v.s.Lookup(typeName, lead); ok && e.Max != 0 {
					allowed = append(allowed, lead)
				}
			}
			v.add(path, "CHOICE_MISSING", "One of "+strings.Join(allowed, ", ")+" is required", "structure", "")
		}
	}
	for _, e := range t.Elements {
		c := counts[e.Name]
		if e.Max != Unbounded && e.Max > 0 && c > e.Max {
			v.add(path+"/"+e.Name, "CARD_MAX", fmt.Sprintf("%s occurs %d times, at most %d allowed", e.Name, c, e.Max), "structure", "")
		}
		if c >= e.Min || e.Max == 0 {
			continue
		}
		if e.Choice != "" {
			// The choice group itself checks that one alternative is present.
			continue
		}
		v.add(path+"/"+e.Name, "CARD_MIN", fmt.Sprintf("%s is required (at least %d)", e.Name, e.Min), "structure", "")
	}
}

func (v *validator) simple(n *Node, path, typeName, fixed string) {
	t := v.s.Simple[typeName]
	val := n.Value()
	if t == nil {
		v.add(path, "STRUCT_UNKNOWN_TYPE", "Unknown simple type "+typeName, "structure", edit(n))
		return
	}
	bad := func(code, msg string) { v.add(path, code, msg, "structure", edit(n)) }
	if fixed != "" && val != fixed {
		bad("FIXED", fmt.Sprintf("Value must be %q", fixed))
	}
	length := utf8.RuneCountInString(val)
	if t.MinLength >= 0 && length < t.MinLength {
		bad("LENGTH_MIN", fmt.Sprintf("At least %d characters", t.MinLength))
	}
	if t.MaxLength >= 0 && length > t.MaxLength {
		bad("LENGTH_MAX", fmt.Sprintf("At most %d characters (%d)", t.MaxLength, length))
	}
	if re := v.s.patterns[typeName]; re != nil && !re.MatchString(val) {
		bad("PATTERN", "Value does not match the expected format "+t.Pattern)
	}
	if t.CodeList != "" && !v.s.HasCode(t.CodeList, val) {
		bad("ENUM", fmt.Sprintf("%q is not a code of %s", val, t.CodeList))
	}
	switch t.Base {
	case BaseInt, BaseDecimal:
		r, ok := new(big.Rat).SetString(val)
		if !ok || strings.ContainsAny(val, "eE/") || (t.Base == BaseInt && strings.Contains(val, ".")) {
			bad("TYPE", "Expected a "+string(t.Base))
			return
		}
		if t.MinInclusive != "" {
			if m, _ := new(big.Rat).SetString(t.MinInclusive); m != nil && r.Cmp(m) < 0 {
				bad("RANGE_MIN", "Minimum is "+t.MinInclusive)
			}
		}
		if t.MaxInclusive != "" {
			if m, _ := new(big.Rat).SetString(t.MaxInclusive); m != nil && r.Cmp(m) > 0 {
				bad("RANGE_MAX", "Maximum is "+t.MaxInclusive)
			}
		}
		digits := strings.TrimLeft(strings.TrimPrefix(strings.TrimPrefix(val, "-"), "+"), "0")
		frac := ""
		if i := strings.IndexByte(digits, '.'); i >= 0 {
			frac = strings.TrimRight(digits[i+1:], "0")
			digits = digits[:i] + frac
		}
		if t.FractionDigits >= 0 && len(frac) > t.FractionDigits {
			bad("FRACTION_DIGITS", fmt.Sprintf("At most %d decimals", t.FractionDigits))
		}
		if t.TotalDigits > 0 && len(digits) > t.TotalDigits {
			bad("TOTAL_DIGITS", fmt.Sprintf("At most %d digits", t.TotalDigits))
		}
	case BaseBoolean:
		if val != "true" && val != "false" && val != "1" && val != "0" {
			bad("TYPE", "Expected true or false")
		}
	case BaseDateTime:
		if _, err := time.Parse("2006-01-02T15:04:05", val); err != nil {
			bad("TYPE", "Expected a date time YYYY-MM-DDThh:mm:ss")
		} else if t.MinInclusive != "" && val < t.MinInclusive {
			bad("RANGE_MIN", "Minimum is "+t.MinInclusive)
		}
	case BaseDate:
		if _, err := time.Parse("2006-01-02", val); err != nil {
			bad("TYPE", "Expected a date YYYY-MM-DD")
		}
	}
}
