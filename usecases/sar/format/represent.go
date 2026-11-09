package format

import (
	"sort"
	"strings"

	"github.com/checkmarble/marble-backend/models"
)

// ViewType is the editing contract of a value for the frontend.
type ViewType struct {
	Base      Base   `json:"base"`
	MinLength int    `json:"min_length,omitempty"`
	MaxLength int    `json:"max_length,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	Min       string `json:"min,omitempty"`
	Max       string `json:"max,omitempty"`
	Fraction  int    `json:"fraction_digits,omitempty"`
	CodeList  string `json:"code_list,omitempty"`
	Fixed     string `json:"fixed,omitempty"`
}

// ViewNode is one node of the report tree: a group (section) or a field.
type ViewNode struct {
	Path       string                `json:"path"`
	Name       string                `json:"name"`
	Label      string                `json:"label"`
	Kind       string                `json:"kind"` // group, field, attribute
	Min        int                   `json:"min"`
	Max        int                   `json:"max"` // -1 unbounded
	Type       *ViewType             `json:"type,omitempty"`
	Value      *string               `json:"value,omitempty"`
	Display    string                `json:"display,omitempty"` // label of a code value
	Provenance *models.SarProvenance `json:"provenance,omitempty"`
	Edit       string                `json:"edit,omitempty"` // pivot/... or extension/... path, empty when not editable
	Derived    string                `json:"derived,omitempty"`
	Missing    bool                  `json:"missing,omitempty"` // required and absent
	Issues     []models.SarIssue     `json:"issues,omitempty"`
	Children   []ViewNode            `json:"children,omitempty"`
}

// EditHint proposes the edit target of a missing element from its
// normalized path (no indexes) and the edit target of its parent.
type EditHint func(normalizedPath, parentEdit string) string

// Represent builds the frontend tree of one projected document. Issues are
// attached to the deepest node whose path prefixes theirs.
func Represent(s *Schema, root *Node, issues []models.SarIssue, hint EditHint) ViewNode {
	s.init()
	byPath := map[string][]models.SarIssue{}
	for _, i := range issues {
		byPath[i.Path] = append(byPath[i.Path], i)
	}
	element := Element{Name: s.RootElement, Type: s.RootType, Min: 1, Max: 1}
	view := represent(s, root, "/"+root.Name, element, byPath, hint, "")
	// Issues on absent paths are attached to their closest existing ancestor.
	attach(&view, byPath)
	return view
}

func label(e Element, s *Schema) string {
	if e.Label != "" {
		return e.Label
	}
	if t := s.Complex[e.Type]; t != nil && t.Label != "" {
		return t.Label
	}
	return strings.ReplaceAll(e.Name, "_", " ")
}

func viewType(s *Schema, e Element) *ViewType {
	name := e.Type
	if t := s.Complex[e.Type]; t != nil {
		if t.Text == "" {
			return nil
		}
		name = t.Text
	}
	t := s.Simple[name]
	if t == nil {
		return nil
	}
	vt := &ViewType{Base: t.Base, Pattern: t.Pattern, Min: t.MinInclusive, Max: t.MaxInclusive, CodeList: t.CodeList, Fixed: e.Fixed}
	if t.MinLength > 0 {
		vt.MinLength = t.MinLength
	}
	if t.MaxLength > 0 {
		vt.MaxLength = t.MaxLength
	}
	if t.FractionDigits > 0 {
		vt.Fraction = t.FractionDigits
	}
	return vt
}

func represent(s *Schema, n *Node, path string, e Element, issues map[string][]models.SarIssue, hint EditHint, parentEdit string) ViewNode {
	v := ViewNode{Path: path, Name: n.Name, Label: label(e, s), Kind: "group", Min: e.Min, Max: e.Max, Type: viewType(s, e)}
	if e.Attribute {
		v.Kind = "attribute"
	} else if v.Type != nil {
		v.Kind = "field"
	}
	if n.Meta != nil {
		v.Provenance, v.Edit, v.Derived = n.Meta.Provenance, n.Meta.Edit, n.Meta.Derived
	}
	if v.Edit == "" {
		v.Edit = parentEdit
	}
	if n.Text != nil {
		val := *n.Text
		v.Value = &val
		if v.Type != nil && v.Type.CodeList != "" {
			v.Display = s.CodeLabel(v.Type.CodeList, val)
		}
	}
	v.Issues = issues[path]
	delete(issues, path)
	t := s.Complex[e.Type]
	if t == nil {
		return v
	}
	counts := map[string]int{}
	present := map[string]bool{}
	for _, c := range n.Children {
		counts[c.Name]++
		present[c.Name] = true
		ce, ok := s.Lookup(e.Type, c.Name)
		if !ok {
			ce = Element{Name: c.Name, Attribute: c.Attribute}
		}
		v.Children = append(v.Children, represent(s, c, ChildPath(path, c, counts[c.Name]), ce, issues, hint, v.Edit))
	}
	order := map[string]int{}
	for i, el := range t.Elements {
		order[el.Name] = i
		if present[el.Name] || el.Min == 0 || el.Max == 0 || el.Choice != "" {
			continue
		}
		mp := path + "/" + el.Name + "[1]"
		if el.Attribute {
			mp = path + "/@" + el.Name
		}
		missing := ViewNode{Path: mp, Name: el.Name, Label: label(el, s), Kind: "field", Min: el.Min, Max: el.Max, Type: viewType(s, el), Missing: true}
		if missing.Type == nil {
			missing.Kind = "group"
		}
		if hint != nil {
			missing.Edit = hint(NormalizePath(mp), v.Edit)
		}
		missing.Issues = append(issues[mp], issues[path+"/"+el.Name]...)
		delete(issues, mp)
		delete(issues, path+"/"+el.Name)
		v.Children = append(v.Children, missing)
	}
	sort.SliceStable(v.Children, func(i, j int) bool {
		a, b := v.Children[i], v.Children[j]
		if a.Kind == "attribute" != (b.Kind == "attribute") {
			return a.Kind == "attribute"
		}
		return order[a.Name] < order[b.Name]
	})
	return v
}

func attach(v *ViewNode, issues map[string][]models.SarIssue) {
	if len(issues) == 0 {
		return
	}
	keys := make([]string, 0, len(issues))
	for k := range issues {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		target := v
		for {
			moved := false
			for i := range target.Children {
				c := &target.Children[i]
				if strings.HasPrefix(k, c.Path+"/") || k == c.Path {
					target, moved = c, true
					break
				}
			}
			if !moved {
				break
			}
		}
		target.Issues = append(target.Issues, issues[k]...)
	}
}

// UsedCodeLists returns the code lists referenced by a view tree.
func UsedCodeLists(s *Schema, views ...ViewNode) map[string]*CodeList {
	out := map[string]*CodeList{}
	var visit func(v ViewNode)
	visit = func(v ViewNode) {
		if v.Type != nil && v.Type.CodeList != "" {
			if l := s.CodeLists[v.Type.CodeList]; l != nil {
				out[v.Type.CodeList] = l
			}
		}
		for _, c := range v.Children {
			visit(c)
		}
	}
	for _, v := range views {
		visit(v)
	}
	return out
}
