package core

import (
	"bytes"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
)

const xmlIndent = "  "

// WriteXML writes the document as XML, walking the spec tree in XSD order
// (spec child order). Rules:
//   - an element is written when it holds a value or a valued descendant, or
//     when it is a mandatory group (min >= 1) under a written parent;
//   - existing items of a repeatable node are always written, in document
//     order; repeatable field items with an empty value are skipped;
//   - nodes with Export false are skipped; attribute nodes become attributes
//     of their parent element; the `_text` child is the element text;
//   - export identifiers (ExportID nodes) are numbered 1, 2, 3... in document
//     order, in ONE sequence shared by every identified item (persons and
//     financial references), so an identifier never designates two elements.
//     The XSD does not constrain id uniqueness (plain t_num_6, no key/keyref);
//     where each id is used (person reference vs financial reference) tells
//     the receiver what it designates. That ERMES accepts ids unique across
//     both families is an unverified assumption;
//   - a ref value (a concrete item path) is written as the identifier of the
//     targeted item; an unresolvable ref is an error. A RefOrText value that
//     is the path of an existing item of its targets is written as that
//     item's id, any other value as the literal text;
//   - dates are written JJ/MM/AAAA, XML 1.0 illegal characters are dropped and
//     text is escaped; no target namespace. On error nothing is written.
func WriteXML(spec *Spec, doc *Document, root string, w io.Writer) error {
	if spec == nil || spec.Root == nil {
		return errors.New("description du format absente")
	}
	x := &xmlWriter{spec: spec, doc: doc, ids: map[Path]int{}, buf: &bytes.Buffer{}}
	x.assignIDs(spec.Root, "")

	x.buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	x.writeElement(root, spec.Root, "", 0)
	if x.err != nil {
		return x.err
	}
	_, err := w.Write(x.buf.Bytes())
	return errors.Wrap(err, "écriture XML")
}

type xmlWriter struct {
	spec *Spec
	doc  *Document
	buf  *bytes.Buffer
	err  error
	ids  map[Path]int // concrete item path -> assigned export id
	seq  int
}

func (x *xmlWriter) fail(err error) {
	if x.err == nil {
		x.err = err
	}
}

// itemPaths lists the concrete paths of a child node under a concrete parent.
func (x *xmlWriter) itemPaths(c *Node, parent Path) []Path {
	base := parent.Child(c.Name)
	if !c.IsRepeatable() {
		return []Path{base}
	}
	keys := x.doc.Items[base]
	out := make([]Path, 0, len(keys))
	for _, k := range keys {
		out = append(out, parent.Item(c.Name, k))
	}
	return out
}

// assignIDs numbers, in document order, every item whose node has an ExportID child.
func (x *xmlWriter) assignIDs(n *Node, concrete Path) {
	for _, c := range n.Children {
		if !c.Export {
			continue
		}
		if c.ExportID {
			x.seq++
			x.ids[concrete] = x.seq
			continue
		}
		if c.Kind == NodeField {
			continue
		}
		for _, p := range x.itemPaths(c, concrete) {
			x.assignIDs(c, p)
		}
	}
}

func (x *xmlWriter) text(raw string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(stripIllegalXMLChars(raw)))
	return b.String()
}

func stripIllegalXMLChars(s string) string {
	ok := true
	for _, r := range s {
		if !isXMLChar(r) {
			ok = false
			break
		}
	}
	if ok {
		return s
	}
	return strings.Map(func(r rune) rune {
		if isXMLChar(r) {
			return r
		}
		return -1
	}, s)
}

func isXMLChar(r rune) bool {
	return r == 0x9 || r == 0xA || r == 0xD ||
		(r >= 0x20 && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0x10FFFF)
}

func (x *xmlWriter) value(p Path) string {
	return x.doc.Values[p].Value
}

// render converts a stored value to its XML form (not yet escaped).
func (x *xmlWriter) render(n *Node, p Path, raw string) (string, bool) {
	switch n.Type {
	case TypeDate:
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			x.fail(errors.Newf("date invalide %q à %s (attendu AAAA-MM-JJ)", raw, p))
			return "", false
		}
		return t.Format("02/01/2006"), true
	case TypeString:
		if n.IsItemRef(raw) {
			if id, ok := x.ids[Path(raw)]; ok {
				return strconv.Itoa(id), true
			}
			// Well-formed path to an item that does not exist: not an id.
			x.fail(errors.Newf("référence %q à %s : élément visé introuvable", raw, p))
			return "", false
		}
	case TypeRef:
		id, ok := x.ids[Path(raw)]
		if !ok {
			x.fail(errors.Newf("référence %q à %s : élément visé introuvable", raw, p))
			return "", false
		}
		return strconv.Itoa(id), true
	}
	return raw, true
}

// hasContent reports whether the node (at the concrete path) holds a value or a
// valued descendant. Existing items count; empty repeatable field items do not.
func (x *xmlWriter) hasContent(n *Node, p Path) bool {
	if !n.Export || n.ExportID {
		return false
	}
	if n.Kind == NodeField {
		return x.value(p) != ""
	}
	for _, c := range n.Children {
		if !c.Export || c.ExportID {
			continue
		}
		for _, cp := range x.itemPaths(c, p) {
			if c.Kind == NodeField {
				if x.value(cp) != "" {
					return true
				}
				continue
			}
			if c.IsRepeatable() || x.hasContent(c, cp) {
				return true
			}
		}
	}
	return false
}

func (x *xmlWriter) indent(depth int) {
	for range depth {
		x.buf.WriteString(xmlIndent)
	}
}

// writeElement writes node n (a group or a field) as element `name`.
func (x *xmlWriter) writeElement(name string, n *Node, p Path, depth int) {
	if x.err != nil {
		return
	}
	if n.Kind == NodeField {
		raw := x.value(p)
		out, ok := x.render(n, p, raw)
		if !ok {
			return
		}
		x.indent(depth)
		x.buf.WriteString("<" + name + ">" + x.text(out) + "</" + name + ">\n")
		return
	}

	var attrs strings.Builder
	var elems []*Node
	var textNode *Node
	for _, c := range n.Children {
		if !c.Export {
			continue
		}
		switch {
		case c.Attribute:
			x.writeAttribute(&attrs, c, p)
		case c.Name == "_text":
			textNode = c
		default:
			elems = append(elems, c)
		}
	}

	var body bytes.Buffer
	outer := x.buf
	x.buf = &body
	for _, c := range elems {
		x.writeChild(c, p, depth+1)
	}
	x.buf = outer

	var text string
	if textNode != nil {
		if raw := x.value(p.Child("_text")); raw != "" {
			if out, ok := x.render(textNode, p.Child("_text"), raw); ok {
				text = x.text(out)
			}
		}
	}

	x.indent(depth)
	switch {
	case body.Len() > 0:
		x.buf.WriteString("<" + name + attrs.String() + ">\n")
		x.buf.Write(body.Bytes())
		x.indent(depth)
		x.buf.WriteString("</" + name + ">\n")
	case text != "":
		x.buf.WriteString("<" + name + attrs.String() + ">" + text + "</" + name + ">\n")
	default:
		x.buf.WriteString("<" + name + attrs.String() + "/>\n")
	}
}

func (x *xmlWriter) writeAttribute(sb *strings.Builder, c *Node, parent Path) {
	if c.ExportID {
		if id, ok := x.ids[parent]; ok {
			sb.WriteString(" " + c.Name + `="` + strconv.Itoa(id) + `"`)
		}
		return
	}
	p := parent.Child(c.Name)
	raw := x.value(p)
	if raw == "" {
		return
	}
	out, ok := x.render(c, p, raw)
	if !ok {
		return
	}
	sb.WriteString(" " + c.Name + `="` + x.text(out) + `"`)
}

// writeChild writes child node c of the concrete parent path.
func (x *xmlWriter) writeChild(c *Node, parent Path, depth int) {
	if x.err != nil || !c.Export {
		return
	}
	if c.ExportID {
		if id, ok := x.ids[parent]; ok {
			x.indent(depth)
			x.buf.WriteString("<" + c.Name + ">" + strconv.Itoa(id) + "</" + c.Name + ">\n")
		}
		return
	}
	for _, cp := range x.itemPaths(c, parent) {
		switch {
		case c.Kind == NodeField:
			if x.value(cp) == "" {
				continue
			}
			x.writeElement(c.Name, c, cp, depth)
		case c.IsRepeatable():
			// Existing items are always written.
			x.writeElement(c.Name, c, cp, depth)
		case x.hasContent(c, cp) || c.Min >= 1:
			x.writeElement(c.Name, c, cp, depth)
		}
	}
}
