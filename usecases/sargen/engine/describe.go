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
	// describeMaxBytes is the hard bound of the « Fields » and « Code lists »
	// sections together. They depend on the number of field templates of the
	// part, not on the number of items, so the bound holds for any part. A
	// template that does not fit loses its description, then is left out.
	describeMaxBytes = 48 * 1024
	// describeInlineCodesMax is the largest code list written in the « Code
	// lists » section; a longer list is named with its size and searched with
	// search_code.
	describeInlineCodesMax = 60
	// describeDescriptionRunes trims the description of a field.
	describeDescriptionRunes = 200
	// describeMaxItemLines caps the « Items » section (item headers and current
	// values), separately from the bound above.
	describeMaxItemLines = 300
)

// DescribeSubtree renders the part of the document entrusted to a task in
// three sections.
//
//   - Fields: one line per field template (`work.transactions[].sens`), written
//     once whatever the number of items, with what the field_spec tool would
//     answer: type, required or not, lengths, pattern, decimals, reference
//     targets, code list name, nature, description trimmed to 200 runes, and for
//     a repeatable node its bounds. Fields the model never writes (setting,
//     computed, analyst, export identifiers) are left out.
//   - Code lists: every list used by the fields, written once as `code = label`
//     lines with its unknown-code rule; a list of more than 60 codes is named
//     with its size and the hint to use search_code. A field line only names its
//     list.
//   - Items: one line per existing item of the part, by key, followed by the
//     fields that already hold a value (the only text wrapped in an
//     UntrustedBlock: the rest comes from the format). An empty list item reads
//     "(empty)".
//
// The Fields and Code lists sections are bounded by describeMaxBytes, the Items
// section by describeMaxItemLines. The output is deterministic, except for the
// random nonce of the UntrustedBlock, drawn once per description.
func DescribeSubtree(spec *core.Spec, doc *core.Document, allowed []core.Path) string {
	return describeSubtree(spec, doc, allowed, describeMaxBytes)
}

func describeSubtree(spec *core.Spec, doc *core.Document, allowed []core.Path, limit int) string {
	if spec == nil || doc == nil {
		return ""
	}
	d := &describer{spec: spec, doc: doc, limit: limit, tmplSeen: map[string]bool{}, listSeen: map[string]bool{}, nonce: newNonce(),
		allowed: allowed, headers: map[string]bool{}}
	d.prefix = commonItemPrefix(allowed)
	seen := map[core.Path]bool{}
	for _, p := range allowed {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		d.entry(p)
	}
	var out []string
	if len(d.fields) > 0 || d.omitted > 0 {
		out = append(out, "### Fields (replace `[]` with the item key, or with a provisional key `[+x]` to create an element)",
			"Every field of this part is described below with the same content field_spec returns, except the lines marked (detail: field_spec): call field_spec only for those or for a field outside this list.")
		out = append(out, d.fields...)
		if d.omitted > 0 {
			out = append(out, fmt.Sprintf("- (%d more fields not described here: use field_spec)", d.omitted))
		}
	}
	if len(d.lists) > 0 {
		out = append(out, "", "### Code lists")
		for _, name := range d.lists {
			out = append(out, d.listSection(name))
		}
	}
	if len(d.items) > 0 || d.itemsCut > 0 {
		out = append(out, "", "### Items")
		out = append(out, d.items...)
		if d.itemsCut > 0 {
			out = append(out, fmt.Sprintf("- (%d more lines not shown: read the values with the tools available)", d.itemsCut))
		}
	}
	return strings.Join(out, "\n")
}

type describer struct {
	spec *core.Spec
	doc  *core.Document
	// limit bounds fields and code lists; used counts their bytes.
	limit int
	used  int
	nonce string

	fields   []string
	omitted  int
	tmplSeen map[string]bool
	// lists are the code lists of the « Code lists » section, in order of first
	// use; listSeen is their set.
	lists    []string
	listSeen map[string]bool
	items    []string
	itemsCut int
	// allowed are the paths of the part; prefix is the item prefix they all
	// share (`personnes.pp[pp-2]`), empty when they do not share one.
	allowed []core.Path
	prefix  core.Path
	headers map[string]bool
}

func (d *describer) addItem(line string) {
	if len(d.items) >= describeMaxItemLines {
		d.itemsCut++
		return
	}
	d.items = append(d.items, line)
}

// entry describes one allowed path: a concrete item or field, or a repeatable
// container without key (its existing items).
func (d *describer) entry(p core.Path) {
	d.itemHeader(itemPrefix(p))
	tmpl := p.Template()
	if n := d.spec.NodeAt(tmpl); n != nil && n.Template != "" {
		d.inner(n, p)
		return
	}
	if n := d.spec.NodeAt(tmpl + "[]"); n != nil && n.IsRepeatable() {
		d.repeatable(n, p)
	}
}

// child describes node n as a child of the concrete path parent.
func (d *describer) child(n *core.Node, parent core.Path) {
	if n.ExportID {
		return
	}
	base := parent.Child(n.Name)
	if n.IsRepeatable() {
		d.repeatable(n, base)
		return
	}
	d.inner(n, base)
}

// repeatable registers the template of the container base and describes its
// existing items.
func (d *describer) repeatable(n *core.Node, base core.Path) {
	if n.ExportID {
		return
	}
	d.templates(n)
	for _, k := range d.doc.Items[base] {
		d.inner(n, core.Path(string(base)+"["+k+"]"))
	}
}

// templates registers the template of n and of everything below it, whether or
// not an item exists yet: the model must see how to write a new item, including
// nested lists (`a[+a].b[+b].c`).
func (d *describer) templates(n *core.Node) {
	if n.ExportID {
		return
	}
	if n.Kind == core.NodeField || n.IsRepeatable() {
		d.template(n)
	}
	for _, c := range n.Children {
		d.templates(c)
	}
}

// inner describes node n at a concrete path.
func (d *describer) inner(n *core.Node, concrete core.Path) {
	if n.ExportID {
		return
	}
	if n.Kind == core.NodeField {
		d.leaf(n, concrete)
		return
	}
	if n.IsRepeatable() {
		d.template(n)
		d.itemHeader(concrete)
	}
	for _, c := range n.Children {
		d.child(c, concrete)
	}
}

// itemHeader writes the « Items » header of an item of the part, once, even
// when none of its fields holds a value yet.
func (d *describer) itemHeader(item core.Path) {
	if item == "" || d.headers[string(item)] {
		return
	}
	n := d.spec.NodeAt(item.Template())
	if n == nil || n.Template == "" {
		return
	}
	d.headers[string(item)] = true
	d.addItem(fmt.Sprintf("- %s — %s %s (fields: %s.*)", item, kindWord(n), label(n), item))
}

func kindWord(n *core.Node) string {
	if n.Kind == core.NodeField {
		return "item"
	}
	return "block"
}

// itemPrefix is the path up to and including its last real item key
// (`personnes.pp[pp-2].etatCivil` -> `personnes.pp[pp-2]`), empty when it has none.
func itemPrefix(p core.Path) core.Path {
	segs := strings.Split(string(p), ".")
	last := -1
	for i, seg := range segs {
		if j := strings.Index(seg, "["); j >= 0 && !strings.HasPrefix(seg[j+1:], "+") {
			last = i
		}
	}
	if last < 0 {
		return ""
	}
	return core.Path(strings.Join(segs[:last+1], "."))
}

// commonItemPrefix is the longest item prefix shared by every allowed path
// (`personnes.pp[pp-2]` for a person with declared financial references, whose
// fields carry their own deeper item prefix), empty when there is none or when
// the paths do not share one (a lot of transactions).
func commonItemPrefix(allowed []core.Path) core.Path {
	var common []string
	first := true
	for _, p := range allowed {
		if p == "" {
			continue
		}
		if itemPrefix(p) == "" {
			return ""
		}
		segs := strings.Split(string(p), ".")
		if first {
			common, first = segs, false
			continue
		}
		n := 0
		for n < len(common) && n < len(segs) && common[n] == segs[n] {
			n++
		}
		common = common[:n]
	}
	return itemPrefix(core.Path(strings.Join(common, ".")))
}

// display is the template as written to the model: under the part's own item
// prefix when there is one (`personnes.pp[pp-2].etatCivil.prenom[]`).
func (d *describer) display(template string) string {
	if d.prefix == "" {
		return template
	}
	pt := d.prefix.Template()
	if template == pt || strings.HasPrefix(template, pt+".") {
		return string(d.prefix) + template[len(pt):]
	}
	return template
}

// newItemForm returns the path to write a new item of the container template,
// built on an allowed path so that it is within the part (existing ancestors keep
// their real key, the other `[]` become `[+a]`, `[+b]`, ...), or "" when the part
// may not create an item of this container.
func (d *describer) newItemForm(template string) string {
	segs := strings.Split(template, ".")
	nth := strings.Count(template, "[]")
	for _, a := range d.allowed {
		asegs := strings.Split(string(a), ".")
		out := make([]string, len(segs))
		same, slot, letter := true, 0, 0
		for i, seg := range segs {
			if !strings.HasSuffix(seg, "[]") {
				out[i] = seg
				same = same && i < len(asegs) && strings.SplitN(asegs[i], "[", 2)[0] == seg
				continue
			}
			name := strings.TrimSuffix(seg, "[]")
			slot++
			key := ""
			if same && slot < nth && i < len(asegs) && strings.SplitN(asegs[i], "[", 2)[0] == name {
				if j := strings.Index(asegs[i], "["); j >= 0 && !strings.HasPrefix(asegs[i][j+1:], "+") {
					key = asegs[i][j+1 : len(asegs[i])-1]
				}
			}
			if key == "" {
				same = false
				key = fmt.Sprintf("+%c", 'a'+rune(letter%26))
				letter++
			}
			out[i] = name + "[" + key + "]"
		}
		cand := strings.Join(out, ".")
		if core.PathAllowed(core.Path(cand), d.allowed) {
			return cand
		}
	}
	return ""
}

// leaf registers the template of a field the model writes and lists its
// current value.
func (d *describer) leaf(n *core.Node, concrete core.Path) {
	if !writable(n) {
		return
	}
	d.template(n)
	v, ok := d.doc.Values[concrete]
	switch {
	case ok && v.Value != "":
		// Only the value is client or analyst text: the rest comes from the format.
		d.addItem(fmt.Sprintf("- %s ; current value: %s", concrete, untrustedBlockNonce("current_value", oneLine(v.Value), d.nonce)))
	case n.IsRepeatable():
		d.addItem(fmt.Sprintf("- %s ; (empty)", concrete))
	}
}

func writable(n *core.Node) bool {
	nature := n.EffectiveNature()
	return nature == core.NatureCopied || nature == core.NatureJudgment
}

// template writes the line of a node once: in full while the bound allows it,
// without description, pattern and detail when it does not, and not at all
// when even that does not fit.
func (d *describer) template(n *core.Node) {
	if d.tmplSeen[n.Template] {
		return
	}
	if n.Kind == core.NodeField && !writable(n) {
		return
	}
	d.tmplSeen[n.Template] = true
	newList := n.CodeList != "" && !d.listSeen[n.CodeList] && d.listAvailable(n.CodeList)
	listBytes := 0
	if newList {
		listBytes = len(d.listSection(n.CodeList)) + 1
	}
	full := d.line(n, true, newList || d.listSeen[n.CodeList])
	if d.used+len(full)+1+listBytes <= d.limit {
		d.fields = append(d.fields, full)
		d.used += len(full) + 1 + listBytes
		if newList {
			d.lists = append(d.lists, n.CodeList)
			d.listSeen[n.CodeList] = true
		}
		return
	}
	short := d.line(n, false, d.listSeen[n.CodeList]) + " ; (detail: field_spec)"
	if d.used+len(short)+1 <= d.limit {
		d.fields = append(d.fields, short)
		d.used += len(short) + 1
		return
	}
	d.omitted++
}

// line renders a template. The short form drops the description, pattern,
// lengths and reference targets but keeps the type, the nature and the way to
// find the codes.
func (d *describer) line(n *core.Node, full, inSection bool) string {
	if n.Kind != core.NodeField {
		return fmt.Sprintf("- %s — block %s, %s ; %s", d.display(n.Template), label(n), bounds(n), d.creation(n.Template))
	}
	parts := []string{fmt.Sprintf("- %s — %s", d.display(n.Template), label(n))}
	if n.Type != "" {
		parts = append(parts, "type "+typeLabel(n.Type))
	}
	if n.Min >= 1 {
		parts = append(parts, "required")
	} else {
		parts = append(parts, "optional")
	}
	if n.IsRepeatable() {
		parts = append(parts, "repeatable, "+bounds(n)+", "+d.creation(n.Template))
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
	}
	if n.CodeList != "" {
		parts = append(parts, d.codeListRef(n.CodeList, inSection))
	}
	if n.EffectiveNature() == core.NatureJudgment {
		parts = append(parts, "nature: judgment (justification and confidence required)")
	} else {
		parts = append(parts, "nature: copied (source required)")
	}
	if full && n.Description != "" {
		parts = append(parts, trimRunes(oneLine(n.Description), describeDescriptionRunes))
	}
	return strings.Join(parts, " ; ")
}

func (d *describer) creation(template string) string {
	if f := d.newItemForm(template); f != "" {
		return "new item: " + f
	}
	return "existing items only"
}

func trimRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// listAvailable tells whether the list can have a « Code lists » entry: known,
// available and not empty.
func (d *describer) listAvailable(name string) bool {
	list, ok := d.spec.CodeLists[name]
	return ok && !list.Unavailable && len(list.Codes) > 0
}

// codeListRef is how a field line refers to its list; the codes are never
// repeated on the line.
func (d *describer) codeListRef(name string, inSection bool) string {
	switch {
	case !d.listAvailable(name):
		return "code list: " + name
	case inSection:
		return "code list: " + name + " (see Code lists)"
	default:
		return "code list: " + name + " (use search_code)"
	}
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
	return untrustedBlockNonce(label, content, newNonce())
}

func newNonce() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	return hex.EncodeToString(b[:])
}

func untrustedBlockNonce(label, content, nonce string) string {
	label = strings.NewReplacer(`"`, "'", "<", "(", ">", ")", "\n", " ").Replace(label)
	content = closingTagRe.ReplaceAllString(content, "&lt;/data")
	return fmt.Sprintf("<data id=%q source=%q>\n%s\n</data id=%q>", nonce, label, content, nonce)
}
