package engine

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Bounds of the part description.
const (
	// describeMaxBytes bounds the whole description. Fields met after the bound
	// is reached fall back to the short one-line form (no length, pattern or
	// code list section) so that the prompt stays bounded.
	describeMaxBytes = 24 * 1024
	// describeInlineCodesMax is the largest code list written in the « Code
	// lists » subsection; a longer list is named with its size and searched
	// with search_code.
	describeInlineCodesMax = 60
	// describeDescriptionRunes trims the description of a field.
	describeDescriptionRunes = 200
)

// DescribeSubtree renders the part of the document entrusted to a task: one
// line per field with what the field_spec tool would answer (path, label, type,
// required or not, minimum and maximum length, pattern, decimals, reference
// targets, code list name, nature, description trimmed to 200 runes) and the
// value already present (the only part wrapped in an UntrustedBlock: the rest
// comes from the format). The code lists used by the part are written once in
// a « Code lists » subsection (`code = label` lines, with the unknown-code
// rule); a list of more than 60 codes is only named with its size and the hint
// to use search_code. It walks the spec templates (not only existing items),
// skips ExportID nodes and fields of nature setting, computed or analyst, shows
// the bounds of repeatable blocks (e.g. "0 to 5") and how to create an item
// (`[+a]`). The output is deterministic and bounded by describeMaxBytes: past
// the bound the remaining fields use the short form (code lists of at most 12
// codes inline on the line, no lengths or pattern).
func DescribeSubtree(spec *core.Spec, doc *core.Document, allowed []core.Path) string {
	return describeSubtree(spec, doc, allowed, describeMaxBytes)
}

func describeSubtree(spec *core.Spec, doc *core.Document, allowed []core.Path, limit int) string {
	if spec == nil || doc == nil {
		return ""
	}
	d := &describer{spec: spec, doc: doc, limit: limit, listSeen: map[string]bool{}}
	seen := map[core.Path]bool{}
	for _, p := range allowed {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		d.entry(p)
	}
	if len(d.lists) > 0 {
		d.lines = append(d.lines, "", "### Code lists")
		for _, name := range d.lists {
			d.lines = append(d.lines, d.listSection(name))
		}
	}
	return strings.Join(d.lines, "\n")
}

type describer struct {
	spec  *core.Spec
	doc   *core.Document
	lines []string
	// limit bounds the bytes written; used counts the lines and the code list
	// sections written so far.
	limit int
	used  int
	// lists are the code lists written in the « Code lists » subsection, in
	// order of first use; listSeen is their set.
	lists    []string
	listSeen map[string]bool
}

func (d *describer) add(line string) {
	d.lines = append(d.lines, line)
	d.used += len(line) + 1
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
		d.fieldLine(n, concrete)
		return
	}
	if n.IsRepeatable() {
		d.add(fmt.Sprintf("- %s — block %s, %s%s",
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

// fieldLine writes the line of a leaf (nothing for a field the model never
// writes): the full form while the bound allows it, else the short form.
func (d *describer) fieldLine(n *core.Node, concrete core.Path) {
	nature := n.EffectiveNature()
	if nature != core.NatureCopied && nature != core.NatureJudgment {
		return
	}
	rich := d.line(n, concrete, true)
	extra := len(rich) + 1
	newList := n.CodeList != "" && !d.listSeen[n.CodeList] && d.listAvailable(n.CodeList)
	if newList {
		extra += len(d.listSection(n.CodeList)) + 1
	}
	if d.used+extra > d.limit {
		d.add(d.line(n, concrete, false))
		return
	}
	d.add(rich)
	if newList {
		d.lists = append(d.lists, n.CodeList)
		d.listSeen[n.CodeList] = true
		d.used += len(d.listSection(n.CodeList)) + 1
	}
}

// line renders a leaf. The full form carries the lengths, pattern, decimals
// and reference targets, and names the code list (its codes are in the « Code
// lists » subsection); the short form is the older compact line.
func (d *describer) line(n *core.Node, concrete core.Path, full bool) string {
	nature := n.EffectiveNature()
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
	if full {
		switch {
		case n.MinLength > 0 && n.MaxLength > 0:
			parts = append(parts, fmt.Sprintf("length %d to %d", n.MinLength, n.MaxLength))
		case n.MaxLength > 0:
			parts = append(parts, fmt.Sprintf("max length %d", n.MaxLength))
		case n.MinLength > 0:
			parts = append(parts, fmt.Sprintf("min length %d", n.MinLength))
		}
		if n.Pattern != "" {
			parts = append(parts, "pattern "+n.Pattern)
		}
		if n.FractionDigits > 0 {
			parts = append(parts, fmt.Sprintf("%d decimals", n.FractionDigits))
		}
		if len(n.RefTargets) > 0 {
			parts = append(parts, "reference to: "+strings.Join(n.RefTargets, ", "))
		}
		if n.RefOrText {
			parts = append(parts, "or free text")
		}
		if n.CodeList != "" {
			parts = append(parts, d.fullCodeList(n.CodeList))
		}
	} else {
		if n.MaxLength > 0 {
			parts = append(parts, fmt.Sprintf("max length %d", n.MaxLength))
		}
		if n.CodeList != "" {
			parts = append(parts, d.codeList(n.CodeList))
		}
	}
	if nature == core.NatureJudgment {
		parts = append(parts, "nature: judgment (justification and confidence required)")
	} else {
		parts = append(parts, "nature: copied (source required)")
	}
	if n.Description != "" {
		parts = append(parts, trimRunes(oneLine(n.Description), describeDescriptionRunes))
	}
	if v, ok := d.doc.Values[concrete]; ok && v.Value != "" {
		// Only the value is client or analyst text: the rest of the line comes
		// from the format.
		parts = append(parts, "current value: "+UntrustedBlock("current_value", oneLine(v.Value)))
	}
	return strings.Join(parts, " ; ")
}

func trimRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// listAvailable tells whether the list has a « Code lists » entry: known,
// available and not empty.
func (d *describer) listAvailable(name string) bool {
	list, ok := d.spec.CodeLists[name]
	return ok && !list.Unavailable && len(list.Codes) > 0
}

// fullCodeList is the reference to a list on a field line of the full form.
func (d *describer) fullCodeList(name string) string {
	if d.listAvailable(name) {
		return "code list: " + name + " (see Code lists)"
	}
	return "code list: " + name
}

// listSection writes a list once: its size, its unknown-code rule, then the
// `code = label` lines when the list is short enough.
func (d *describer) listSection(name string) string {
	list := d.spec.CodeLists[name]
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d codes)", name, len(list.Codes))
	if list.UnknownCode != "" {
		b.WriteString(" ; " + unknownCodeRule(list))
	}
	if len(list.Codes) > describeInlineCodesMax {
		b.WriteString(" ; too long to list: use search_code to find a code")
		return b.String()
	}
	for _, c := range list.Codes {
		b.WriteString("\n" + c.Value + " = " + oneLine(c.Label))
	}
	return b.String()
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
