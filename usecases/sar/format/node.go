package format

import (
	"strconv"
	"strings"

	"github.com/checkmarble/marble-backend/models"
)

// Meta links a projected value back to what the analyst can edit.
type Meta struct {
	Provenance *models.SarProvenance `json:"provenance,omitempty"`
	// Edit is the pivot or extension path that produces the value
	// (pivot/operations/op1/amount, extension/goaml:LU:5.0.2/report_code).
	Edit string `json:"edit,omitempty"`
	// Derived explains a backend computation (aggregation, conversion, format).
	Derived string `json:"derived,omitempty"`
}

// Node is one element of a projected document. Attributes are nodes with
// Attribute set; elements with simple content carry Text.
type Node struct {
	Name      string  `json:"name"`
	Attribute bool    `json:"attribute,omitempty"`
	Text      *string `json:"text,omitempty"`
	Children  []*Node `json:"children,omitempty"`
	Meta      *Meta   `json:"meta,omitempty"`
}

func NewNode(name string) *Node { return &Node{Name: name} }

// Add appends and returns a child element.
func (n *Node) Add(name string) *Node {
	c := &Node{Name: name}
	n.Children = append(n.Children, c)
	return c
}

// Leaf appends a simple element when value is non-empty.
func (n *Node) Leaf(name, value string, meta *Meta) *Node {
	if value == "" {
		return nil
	}
	v := value
	c := &Node{Name: name, Text: &v, Meta: meta}
	n.Children = append(n.Children, c)
	return c
}

// Attr sets an attribute when value is non-empty.
func (n *Node) Attr(name, value string, meta *Meta) {
	if value == "" {
		return
	}
	v := value
	n.Children = append(n.Children, &Node{Name: name, Attribute: true, Text: &v, Meta: meta})
}

// SetText sets simple content.
func (n *Node) SetText(value string, meta *Meta) {
	v := value
	n.Text = &v
	if meta != nil {
		n.Meta = meta
	}
}

func (n *Node) Value() string {
	if n == nil || n.Text == nil {
		return ""
	}
	return *n.Text
}

// All returns the children with the given name.
func (n *Node) All(name string) []*Node {
	if n == nil {
		return nil
	}
	var out []*Node
	for _, c := range n.Children {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}

// Get follows a slash separated path of child names (first match).
func (n *Node) Get(path string) *Node {
	cur := n
	for _, p := range strings.Split(path, "/") {
		if p == "" {
			continue
		}
		var next *Node
		for _, c := range cur.Children {
			if c.Name == p {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

func (n *Node) Has(path string) bool { return n.Get(path) != nil }
func (n *Node) Str(path string) string {
	if c := n.Get(path); c != nil {
		return c.Value()
	}
	return ""
}

// Walk visits nodes depth first with their stable path.
func Walk(root *Node, fn func(n *Node, path string, parent *Node)) {
	var visit func(n *Node, path string, parent *Node)
	visit = func(n *Node, path string, parent *Node) {
		fn(n, path, parent)
		counts := map[string]int{}
		for _, c := range n.Children {
			counts[c.Name]++
			visit(c, ChildPath(path, c, counts[c.Name]), n)
		}
	}
	visit(root, "/"+root.Name, nil)
}

// ChildPath builds the stable path of the i-th (1-based) child named c.Name.
func ChildPath(parent string, c *Node, i int) string {
	if c.Attribute {
		return parent + "/@" + c.Name
	}
	return parent + "/" + c.Name + "[" + strconv.Itoa(i) + "]"
}

// NormalizePath removes occurrence indexes, for memory deduplication.
func NormalizePath(path string) string {
	var b strings.Builder
	skip := false
	for _, r := range path {
		switch {
		case r == '[':
			skip = true
		case r == ']':
			skip = false
		case !skip:
			b.WriteRune(r)
		}
	}
	return b.String()
}
