package engine

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// DescribeSubtree renders the part of the document entrusted to a task: one
// line per field with path, label, type, required or not, maximum length, code
// list name (inline codes when the list has at most 12 codes), nature
// (copied / judgment), description and the value already present (the only
// part wrapped in an UntrustedBlock: the rest comes from the format). It walks the
// spec templates (not only existing items), skips ExportID nodes and fields of
// nature setting, computed or analyst, shows the bounds of repeatable blocks
// (e.g. "0 to 5") and how to create an item (`[+a]`). The field detail stays
// available through the field_spec tool.
func DescribeSubtree(spec *core.Spec, doc *core.Document, allowed []core.Path) string {
	if spec == nil || doc == nil {
		return ""
	}
	d := &describer{spec: spec, doc: doc}
	seen := map[core.Path]bool{}
	for _, p := range allowed {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		d.entry(p)
	}
	return strings.Join(d.lines, "\n")
}

type describer struct {
	spec  *core.Spec
	doc   *core.Document
	lines []string
}

// entry describes one allowed path: a concrete item or field, or a repeatable
// container without key (existing items and creation).
func (d *describer) entry(p core.Path) {
	tmpl := p.Template()
	if n := d.spec.NodeAt(tmpl); n != nil && n.Template != "" {
		d.inner(n, p, 0)
		return
	}
	if n := d.spec.NodeAt(tmpl + "[]"); n != nil && n.IsRepeatable() {
		d.repeatable(n, p, 0)
	}
}

// child describes node n as a child of the concrete path parent.
func (d *describer) child(n *core.Node, parent core.Path, depth int) {
	if n.ExportID {
		return
	}
	base := parent.Child(n.Name)
	if n.IsRepeatable() {
		d.repeatable(n, base, depth)
		return
	}
	d.inner(n, base, depth)
}

// repeatable describes the existing items of the container base, then the
// creation of a new one with a provisional key.
func (d *describer) repeatable(n *core.Node, base core.Path, depth int) {
	if n.ExportID {
		return
	}
	keys := d.doc.Items[base]
	for _, k := range keys {
		d.inner(n, core.Path(string(base)+"["+k+"]"), depth)
	}
	if n.Max < 0 || len(keys) < n.Max {
		letter := string(rune('a' + depth%26))
		d.inner(n, core.Path(string(base)+"[+"+letter+"]"), depth+1)
	}
}

// inner describes node n at the concrete (or provisional) path.
func (d *describer) inner(n *core.Node, concrete core.Path, depth int) {
	if n.ExportID {
		return
	}
	if n.Kind == core.NodeField {
		if line, ok := d.fieldLine(n, concrete); ok {
			d.lines = append(d.lines, line)
		}
		return
	}
	if n.IsRepeatable() {
		d.lines = append(d.lines, fmt.Sprintf("- %s — block %s, %s%s",
			concrete, label(n), bounds(n), creation(concrete)))
	}
	for _, c := range n.Children {
		d.child(c, concrete, depth)
	}
}

func creation(p core.Path) string {
	if p.HasProvisionalKey() {
		return " ; element to create: use this provisional key in the paths (same key for the same element)"
	}
	return ""
}

func label(n *core.Node) string {
	if n.Label != "" {
		return n.Label
	}
	return n.Name
}

func bounds(n *core.Node) string {
	if n.Max < 0 {
		return fmt.Sprintf("%d to n elements", n.Min)
	}
	return fmt.Sprintf("%d to %d", n.Min, n.Max)
}

func (d *describer) fieldLine(n *core.Node, concrete core.Path) (string, bool) {
	nature := n.EffectiveNature()
	if nature != core.NatureCopied && nature != core.NatureJudgment {
		return "", false
	}
	parts := []string{fmt.Sprintf("- %s — %s", concrete, label(n))}
	if n.Type != "" {
		parts = append(parts, "type "+typeLabel(n.Type))
	}
	if n.Min >= 1 {
		parts = append(parts, "required")
	} else {
		parts = append(parts, "optional")
	}
	if n.IsRepeatable() {
		parts = append(parts, "repeatable, "+bounds(n))
	}
	if n.MaxLength > 0 {
		parts = append(parts, fmt.Sprintf("max length %d", n.MaxLength))
	}
	if n.CodeList != "" {
		parts = append(parts, d.codeList(n.CodeList))
	}
	if nature == core.NatureJudgment {
		parts = append(parts, "nature: judgment (justification and confidence required)")
	} else {
		parts = append(parts, "nature: copied (source required)")
	}
	if n.Description != "" {
		parts = append(parts, oneLine(n.Description))
	}
	if v, ok := d.doc.Values[concrete]; ok && v.Value != "" {
		// Only the value is client or analyst text: the rest of the line comes
		// from the format.
		parts = append(parts, "current value: "+UntrustedBlock("current_value", oneLine(v.Value)))
	}
	return strings.Join(parts, " ; "), true
}

func typeLabel(t core.FieldType) string {
	switch t {
	case core.TypeDate:
		return "date YYYY-MM-DD"
	case core.TypeDecimal:
		return "decimal (dot)"
	case core.TypeInteger:
		return "integer"
	case core.TypeBoolean:
		return "boolean (true/false)"
	case core.TypeCode:
		return "code"
	case core.TypeRef:
		return "reference to an element"
	default:
		return string(t)
	}
}

func (d *describer) codeList(name string) string {
	list, ok := d.spec.CodeLists[name]
	if !ok || list.Unavailable || len(list.Codes) == 0 {
		return "code list: " + name
	}
	desc := "code list: " + name
	if len(list.Codes) <= inlineCodesMax {
		codes := make([]string, len(list.Codes))
		for i, c := range list.Codes {
			codes[i] = c.Value + " (" + oneLine(c.Label) + ")"
		}
		desc += " [" + strings.Join(codes, ", ") + "]"
	}
	if list.UnknownCode != "" {
		desc += " ; " + unknownCodeRule(list)
	}
	return desc
}

// unknownCodeRule is the rule shown to the model for a list with an unknown code.
func unknownCodeRule(list core.CodeList) string {
	rule := fmt.Sprintf("unknown code %q: use it when the data say nothing", list.UnknownCode)
	if len(list.StrictCodes) > 0 {
		quoted := make([]string, len(list.StrictCodes))
		for i, c := range list.StrictCodes {
			quoted[i] = fmt.Sprintf("%q", c)
		}
		rule += "; " + strings.Join(quoted, ", ") + " needs a source that establishes it (its raw value is that code or its label, or a boolean field), else it becomes a question for the analyst"
	}
	return rule
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

var closingTagRe = regexp.MustCompile(`(?i)</\s*data`)

// UntrustedBlock wraps client or case text inserted in a prompt:
//
//	<data id="<nonce>" source="<label>">…</data id="<nonce>">
//
// The nonce is random per call; any occurrence of the closing tag in content
// is neutralised.
func UntrustedBlock(label, content string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	nonce := hex.EncodeToString(b[:])
	label = strings.NewReplacer(`"`, "'", "<", "(", ">", ")", "\n", " ").Replace(label)
	content = closingTagRe.ReplaceAllString(content, "&lt;/data")
	return fmt.Sprintf("<data id=%q source=%q>\n%s\n</data id=%q>", nonce, label, content, nonce)
}
