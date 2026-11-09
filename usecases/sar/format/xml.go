package format

import (
	"bytes"
	"encoding/xml"
	"io"
	"sort"
)

// WriteXML serializes a projected document. Children are ordered by the
// schema sequence of their owner type, attributes first; repeated elements
// keep their projection order.
func WriteXML(w io.Writer, s *Schema, root *Node) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	if err := writeNode(w, s, root, s.RootType, 0, s.Namespace); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func XMLBytes(s *Schema, root *Node) ([]byte, error) {
	var b bytes.Buffer
	err := WriteXML(&b, s, root)
	return b.Bytes(), err
}

func indent(w io.Writer, depth int) error {
	_, err := io.WriteString(w, "\n"+string(bytes.Repeat([]byte("  "), depth)))
	return err
}

func escape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func writeNode(w io.Writer, s *Schema, n *Node, typeName string, depth int, ns string) error {
	if depth > 0 {
		if err := indent(w, depth); err != nil {
			return err
		}
	}
	start := "<" + n.Name
	if ns != "" {
		start += ` xmlns="` + escape(ns) + `"`
	}
	order := map[string]int{}
	if t := s.Complex[typeName]; t != nil {
		for i, e := range t.Elements {
			order[e.Name] = i
		}
	}
	children := make([]*Node, 0, len(n.Children))
	for _, c := range n.Children {
		if c.Attribute {
			start += " " + c.Name + `="` + escape(c.Value()) + `"`
		} else {
			children = append(children, c)
		}
	}
	sort.SliceStable(children, func(i, j int) bool { return order[children[i].Name] < order[children[j].Name] })
	if len(children) == 0 {
		if n.Text == nil {
			_, err := io.WriteString(w, start+"/>")
			return err
		}
		_, err := io.WriteString(w, start+">"+escape(*n.Text)+"</"+n.Name+">")
		return err
	}
	if _, err := io.WriteString(w, start+">"); err != nil {
		return err
	}
	for _, c := range children {
		childType := ""
		if e, ok := s.Lookup(typeName, c.Name); ok {
			childType = e.Type
		}
		if err := writeNode(w, s, c, childType, depth+1, ""); err != nil {
			return err
		}
	}
	if err := indent(w, depth); err != nil {
		return err
	}
	_, err := io.WriteString(w, "</"+n.Name+">")
	return err
}
