package core

import (
	"github.com/cockroachdb/errors"
	"slices"
	"strings"
)

type NodeKind string

const (
	NodeGroup  NodeKind = "group"
	NodeField  NodeKind = "field"
	NodeChoice NodeKind = "choice"
)

type FieldType string

const (
	TypeString  FieldType = "string"
	TypeDate    FieldType = "date" // stored AAAA-MM-JJ, written JJ/MM/AAAA
	TypeDecimal FieldType = "decimal"
	TypeInteger FieldType = "integer"
	TypeBoolean FieldType = "boolean"
	TypeCode    FieldType = "code"
	TypeRef     FieldType = "ref"
)

type Nature string

const (
	NatureCopied   Nature = "copied" // default
	NatureJudgment Nature = "judgment"
	NatureSetting  Nature = "setting"
	NatureComputed Nature = "computed"
	NatureAnalyst  Nature = "analyst"
)

type Node struct {
	Name      string   `json:"name"`
	Template  string   `json:"template"`
	Kind      NodeKind `json:"kind"`
	Min       int      `json:"min"`
	Max       int      `json:"max"` // -1 = unbounded
	Attribute bool     `json:"attribute,omitempty"`
	Export    bool     `json:"export"` // false = working node, never written to XML
	// ExportID marks an identifier node (element or attribute) that the XML writer
	// fabricates from internal item keys: it never holds a document value.
	ExportID       bool      `json:"export_id,omitempty"`
	Type           FieldType `json:"type,omitempty"`
	Nature         Nature    `json:"nature,omitempty"`
	MinLength      int       `json:"min_length,omitempty"` // in characters
	MaxLength      int       `json:"max_length,omitempty"`
	Pattern        string    `json:"pattern,omitempty"`
	FractionDigits int       `json:"fraction_digits,omitempty"`
	CodeList       string    `json:"code_list,omitempty"`
	RefTargets     []string  `json:"ref_targets,omitempty"`
	// RefOrText: the value is either the path of an existing item of RefTargets
	// (written as its export id) or free text. The type stays TypeString.
	RefOrText   bool    `json:"ref_or_text,omitempty"`
	Label       string  `json:"label,omitempty"`
	Description string  `json:"description,omitempty"`
	Children    []*Node `json:"children,omitempty"`
}

type Code struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type CodeList struct {
	Name        string `json:"name"`
	Codes       []Code `json:"codes,omitempty"`
	Unavailable bool   `json:"unavailable,omitempty"`
	Source      string `json:"source,omitempty"`
}

type Condition struct {
	Path   string   `json:"path"`
	Op     string   `json:"op"` // eq, in, not_in, present, absent
	Values []string `json:"values,omitempty"`
}

type Rule struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Scope       string     `json:"scope,omitempty"`
	Description string     `json:"description,omitempty"`
	Source      string     `json:"source,omitempty"`
	Severity    Severity   `json:"severity,omitempty"` // default blocking
	Then        []string   `json:"then,omitempty"`
	When        *Condition `json:"when,omitempty"`
	Among       []string   `json:"among,omitempty"`
}

type FormatKey struct {
	Family  string `json:"family"`
	Country string `json:"country"`
	Version string `json:"version"`
}

func (k FormatKey) String() string {
	return k.Family + ":" + k.Country + ":" + k.Version
}

func ParseFormatKey(s string) (FormatKey, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return FormatKey{}, errors.Newf("clé de format invalide %q (attendu famille:pays:version)", s)
	}
	return FormatKey{Family: parts[0], Country: parts[1], Version: parts[2]}, nil
}

type InputPin struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Version string `json:"version,omitempty"`
}

type Spec struct {
	Key         FormatKey           `json:"key"`
	ReportTypes []string            `json:"report_types,omitempty"`
	Root        *Node               `json:"root"`
	CodeLists   map[string]CodeList `json:"code_lists,omitempty"`
	Rules       []Rule              `json:"rules,omitempty"`
	Inputs      []InputPin          `json:"inputs,omitempty"`
	// Defaults maps a template to the raw value set when a declaration is created.
	Defaults map[string]string `json:"defaults,omitempty"`
}

// NodeAt returns the node with the given template (e.g. `personnes.pp[]`), or nil.
// The root has an empty template.
func (s *Spec) NodeAt(template string) *Node {
	if s == nil || s.Root == nil {
		return nil
	}
	if template == "" {
		return s.Root
	}
	return findNode(s.Root, template)
}

func findNode(n *Node, template string) *Node {
	for _, c := range n.Children {
		if c.Template == template {
			return c
		}
		if strings.HasPrefix(template, c.Template+".") {
			if found := findNode(c, template); found != nil {
				return found
			}
		}
	}
	return nil
}

func (n *Node) repeatable() bool { return n.Max < 0 || n.Max > 1 }

// Leaves returns the concrete leaf paths under prefix, following the items
// that exist in doc. Order is the spec child order, items in key order.
func (s *Spec) Leaves(prefix Path, doc *Document) []Path {
	var out []Path
	tmpl := prefix.Template()
	node := s.NodeAt(tmpl)
	if node == nil {
		// Container path given without its key segment.
		if n := s.NodeAt(tmpl + "[]"); n != nil && n.repeatable() {
			for _, key := range doc.Items[prefix] {
				out = visitLeaves(n, Path(string(prefix)+"["+key+"]"), doc, out)
			}
		}
		return out
	}
	if node.Template == "" {
		for _, c := range node.Children {
			out = expandLeaves(c, "", doc, out)
		}
		return out
	}
	return visitLeaves(node, prefix, doc, out)
}

// expandLeaves handles node n as a child of parent, expanding repeatable nodes.
func expandLeaves(n *Node, parent Path, doc *Document, out []Path) []Path {
	base := parent.Child(n.Name)
	if n.repeatable() {
		for _, key := range doc.Items[base] {
			out = visitLeaves(n, Path(string(base)+"["+key+"]"), doc, out)
		}
		return out
	}
	return visitLeaves(n, base, doc, out)
}

func visitLeaves(n *Node, concrete Path, doc *Document, out []Path) []Path {
	if n.Kind == NodeField {
		return append(out, concrete)
	}
	for _, c := range n.Children {
		out = expandLeaves(c, concrete, doc, out)
	}
	return out
}

// EffectiveNature returns the node nature, defaulting to NatureCopied.
func (n *Node) EffectiveNature() Nature {
	if n.Nature == "" {
		return NatureCopied
	}
	return n.Nature
}

// EffectiveSeverity returns the rule severity, defaulting to SeverityBlocking.
func (r Rule) EffectiveSeverity() Severity {
	if r.Severity == "" {
		return SeverityBlocking
	}
	return r.Severity
}

// IsItemRef reports whether raw is, for a RefOrText node, the path of an item
// whose template is one of RefTargets (otherwise it is free text).
func (n *Node) IsItemRef(raw string) bool {
	if !n.RefOrText {
		return false
	}
	if _, err := ParsePath(raw); err != nil {
		return false
	}
	return slices.Contains(n.RefTargets, Path(raw).Template())
}
