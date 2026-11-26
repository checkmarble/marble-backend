package core

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Issue codes produced by the generic validator.
const (
	CodeUnknownPath         = "unknown_path"
	CodeUnknownItem         = "unknown_item"
	CodeNotAField           = "not_a_field"
	CodeExportIDValue       = "export_id_value"
	CodeRequired            = "required"
	CodeTooFewItems         = "too_few_items"
	CodeTooManyItems        = "too_many_items"
	CodeBadDate             = "bad_date"
	CodeBadBoolean          = "bad_boolean"
	CodeBadInteger          = "bad_integer"
	CodeBadDecimal          = "bad_decimal"
	CodeBadRef              = "bad_ref"
	CodeDanglingRef         = "dangling_ref"
	CodeMaxLength           = "max_length"
	CodeMinLength           = "min_length"
	CodePattern             = "pattern"
	CodeUnknownCode         = "unknown_code"
	CodeFreeTextCode        = "free_text_code"
	CodeBadChar             = "bad_char"
	CodeBadPattern          = "bad_pattern"
	CodeCodeListUnavailable = "code_list_unavailable"
	CodeUnknownRule         = "unknown_rule"
	ruleCodePrefix          = "rule:"
	ruleKindRequiredIf      = "required_if"
	ruleKindForbiddenIf     = "forbidden_if"
	ruleKindExactlyOne      = "exactly_one"
	ruleKindAtMostOne       = "at_most_one"
	opEq, opIn, opNotIn     = "eq", "in", "not_in"
	opPresent, opAbsent     = "present", "absent"
	dateLayout              = "2006-01-02"
	unitCharacters          = "characters"
	unitDigits              = "digits"
)

var (
	decimalRe  = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)
	integerRe  = regexp.MustCompile(`^[0-9]+$`)
	patternMu  sync.RWMutex
	patternMap = map[string]*regexp.Regexp{}
)

// anchoredPattern compiles `^(?:p)$` once per pattern. It returns nil when the
// pattern does not compile (the generator and a test guarantee it does).
func anchoredPattern(p string) *regexp.Regexp {
	patternMu.RLock()
	re, ok := patternMap[p]
	patternMu.RUnlock()
	if ok {
		return re
	}
	patternMu.Lock()
	defer patternMu.Unlock()
	re, err := regexp.Compile("^(?:" + p + ")$")
	if err != nil {
		re = nil
	}
	patternMap[p] = re
	return re
}

// hasIllegalXMLChar reports a value that XML 1.0 cannot carry: an invalid
// UTF-8 sequence or a rune outside the XML 1.0 Char production.
func hasIllegalXMLChar(s string) bool {
	if !utf8.ValidString(s) {
		return true
	}
	for _, r := range s {
		if !isXMLChar(r) {
			return true
		}
	}
	return false
}

func blocking(path Path, code, msg string, params ...string) *Issue {
	return &Issue{Path: path, Code: code, Message: msg, Severity: SeverityBlocking, Params: Params(params...)}
}

func itoa(n int) string { return strconv.Itoa(n) }

// SortIssues orders issues by path, then code, then rule id.
func SortIssues(issues []Issue) {
	sort.SliceStable(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.RuleID < b.RuleID
	})
}

// ValidateValue checks one raw value against the node at path: structure
// (path known, a field, not an identifier node), type, length, pattern, code
// list. It knows nothing of the rest of the document. An empty value is
// accepted (it clears the field). The returned issue may be a warning (for
// example code_list_unavailable): callers must use Issue.Blocks to decide
// whether to refuse the value.
func ValidateValue(spec *Spec, path Path, raw string) *Issue {
	if _, err := ParsePath(string(path)); err != nil {
		return blocking(path, CodeUnknownPath, "invalid path: "+err.Error())
	}
	node := spec.NodeAt(path.Template())
	if node == nil || node.Template == "" {
		return blocking(path, CodeUnknownPath, "unknown field in the format")
	}
	if node.ExportID {
		return blocking(path, CodeExportIDValue, "this identifier is generated at export: no value to enter")
	}
	if node.Kind != NodeField {
		return blocking(path, CodeNotAField, "this path designates a group, not a field")
	}
	if raw == "" {
		return nil
	}
	if hasIllegalXMLChar(raw) {
		return blocking(path, CodeBadChar, "character not allowed in XML (control character or invalid UTF-8 sequence)")
	}
	return checkValue(spec, node, path, raw)
}

func checkValue(spec *Spec, node *Node, path Path, raw string) *Issue {
	switch node.Type {
	case TypeBoolean:
		if raw != "true" && raw != "false" {
			return blocking(path, CodeBadBoolean, "expected value: true or false")
		}
		return nil
	case TypeDate:
		if _, err := time.Parse(dateLayout, raw); err != nil {
			return blocking(path, CodeBadDate, "invalid date, expected format YYYY-MM-DD", "expected", "YYYY-MM-DD")
		}
		return nil
	case TypeInteger:
		if !integerRe.MatchString(raw) {
			return blocking(path, CodeBadInteger, "integer expected")
		}
		// totalDigits counts the digits of the value: leading zeros do not count.
		if node.MaxLength > 0 && len(strings.TrimLeft(raw, "0")) > node.MaxLength {
			return blocking(path, CodeMaxLength, fmt.Sprintf("at most %d digits", node.MaxLength),
				"max", itoa(node.MaxLength), "unit", unitDigits)
		}
		return checkPattern(node, path, raw)
	case TypeDecimal:
		if issue := checkDecimal(node, path, raw); issue != nil {
			return issue
		}
		return checkPattern(node, path, raw)
	case TypeCode:
		return checkCode(spec, node, path, raw)
	case TypeRef:
		// A reference names one item: a template such as `personnes.pp[]` (no key)
		// is refused here, not only later by the whole-document check.
		if _, err := ParsePath(raw); err != nil || !slices.Contains(node.RefTargets, Path(raw).Template()) {
			return blocking(path, CodeBadRef, "reference to an item of a type that is not allowed", "value", raw)
		}
		return nil
	}
	if node.isWrongKindRef(spec, raw) {
		return blocking(path, CodeBadRef, "reference to an item of a type that is not allowed", "value", raw)
	}
	if node.IsItemRef(raw) {
		// Id of a declared item: existence is checked against the document.
		return nil
	}
	return checkFacets(node, path, raw)
}

func checkPattern(node *Node, path Path, raw string) *Issue {
	if node.Pattern == "" {
		return nil
	}
	re := anchoredPattern(node.Pattern)
	if re == nil {
		// Fail closed: a pattern we cannot read must not let a value through.
		return blocking(path, CodeBadPattern, "unreadable format pattern: "+node.Pattern, "pattern", node.Pattern)
	}
	if !re.MatchString(raw) {
		return blocking(path, CodePattern, "invalid value format")
	}
	return nil
}

// checkFacets applies the length and pattern facets common to strings and codes.
func checkFacets(node *Node, path Path, raw string) *Issue {
	n := utf8.RuneCountInString(raw)
	if node.MinLength > 0 && n < node.MinLength {
		return blocking(path, CodeMinLength, fmt.Sprintf("at least %d characters (%d present)", node.MinLength, n),
			"min", itoa(node.MinLength), "present", itoa(n), "unit", unitCharacters)
	}
	if node.MaxLength > 0 && n > node.MaxLength {
		return blocking(path, CodeMaxLength, fmt.Sprintf("at most %d characters (%d present)", node.MaxLength, n),
			"max", itoa(node.MaxLength), "present", itoa(n), "unit", unitCharacters)
	}
	return checkPattern(node, path, raw)
}

func checkDecimal(node *Node, path Path, raw string) *Issue {
	if !decimalRe.MatchString(raw) {
		return blocking(path, CodeBadDecimal, "decimal number expected (e.g. 1234.56)")
	}
	digits := strings.TrimPrefix(raw, "-")
	intPart, frac, _ := strings.Cut(digits, ".")
	// totalDigits and fractionDigits apply to the value: leading zeros of the
	// integer part and trailing zeros of the fraction do not count.
	intPart = strings.TrimLeft(intPart, "0")
	frac = strings.TrimRight(frac, "0")
	if node.FractionDigits > 0 && len(frac) > node.FractionDigits {
		return blocking(path, CodeBadDecimal, fmt.Sprintf("at most %d fraction digits", node.FractionDigits),
			"max_fraction_digits", itoa(node.FractionDigits))
	}
	if node.MaxLength > 0 && len(intPart)+len(frac) > node.MaxLength {
		return blocking(path, CodeMaxLength, fmt.Sprintf("at most %d digits", node.MaxLength),
			"max", itoa(node.MaxLength), "unit", unitDigits)
	}
	return nil
}

func checkCode(spec *Spec, node *Node, path Path, raw string) *Issue {
	list, ok := spec.CodeLists[node.CodeList]
	if !ok {
		return blocking(path, CodeUnknownCode, "unknown code list: "+node.CodeList, "code_list", node.CodeList, "value", raw)
	}
	if issue := checkFacets(node, path, raw); issue != nil {
		// Length and pattern facets of the XSD type also bind code values.
		return issue
	}
	if list.Unavailable {
		return &Issue{
			Path: path, Code: CodeCodeListUnavailable, Severity: SeverityWarning,
			Message: "code list not published: value not checked (" + node.CodeList + ")",
			Params:  Params("code_list", node.CodeList),
		}
	}
	if !node.IsListedCode(spec, raw) {
		if node.CodeOrText {
			return &Issue{
				Path: path, Code: CodeFreeTextCode, Severity: SeverityWarning,
				Message: "not a code of " + node.CodeList + ": accepted as free text, to check (typo, value really missing from the list)",
				Params:  Params("code_list", node.CodeList, "value", raw),
			}
		}
		return blocking(path, CodeUnknownCode, "unknown code in list "+node.CodeList, "code_list", node.CodeList, "value", raw)
	}
	return nil
}

type validator struct {
	spec      *Spec
	doc       *Document
	ancestors map[Path]bool // paths with a value or an item below (or at)
	valued    map[Path]bool // paths with a non-empty value below (or at)
	issues    []Issue
}

// Validate checks the whole document against the spec and its generic rules.
// Issues are ordered by path, then code.
func Validate(spec *Spec, doc *Document) []Issue {
	if spec == nil {
		return nil
	}
	v := &validator{spec: spec, doc: doc, ancestors: map[Path]bool{}, valued: map[Path]bool{}}
	v.indexAncestors()
	v.checkValues()
	if spec.Root != nil {
		v.checkChildren(spec.Root, "", true)
	}
	for _, r := range spec.Rules {
		v.checkRule(r)
	}
	SortIssues(v.issues)
	return v.issues
}

func (v *validator) add(i *Issue) {
	if i != nil {
		v.issues = append(v.issues, *i)
	}
}

func (v *validator) addAncestors(p Path) {
	for p != "" && !v.ancestors[p] {
		v.ancestors[p] = true
		p = p.Parent()
	}
}

func (v *validator) indexAncestors() {
	for p, val := range v.doc.Values {
		if val.Value != "" {
			v.addAncestors(p)
			for q := p; q != ""; q = q.Parent() {
				v.valued[q] = true
			}
		}
	}
	for _, container := range v.doc.ItemContainers() {
		// Items of a repeatable field container with an empty value are skipped by
		// the writer: they must not make their ancestor groups present.
		isField := false
		if node := v.spec.NodeAt(container.Template() + "[]"); node != nil && node.Kind == NodeField {
			isField = true
		}
		for _, k := range v.doc.ItemKeys(container) {
			item := Path(string(container) + "[" + k + "]")
			if isField && !v.hasValue(item) {
				continue
			}
			v.addAncestors(item)
		}
	}
}

// itemsPresent checks every keyed segment of the path points to an existing item.
func (v *validator) itemsPresent(p Path) bool {
	segs := strings.Split(string(p), ".")
	for i, seg := range segs {
		if strings.HasSuffix(seg, "]") && !v.doc.HasItem(Path(strings.Join(segs[:i+1], "."))) {
			return false
		}
	}
	return true
}

func (v *validator) checkValues() {
	for p, val := range v.doc.Values {
		if val.Value == "" {
			continue
		}
		if _, err := ParsePath(string(p)); err == nil && !v.itemsPresent(p) {
			v.add(blocking(p, CodeUnknownItem, "the item containing this value does not exist"))
			continue
		}
		issue := ValidateValue(v.spec, p, val.Value)
		if issue != nil {
			v.add(issue)
			continue
		}
		// The target and every item above it must exist: an account under a
		// missing person has no export id.
		if node := v.spec.NodeAt(p.Template()); node != nil && (node.Type == TypeRef || node.IsItemRef(val.Value)) && !v.itemsPresent(Path(val.Value)) {
			v.add(blocking(p, CodeDanglingRef, "the reference targets a missing item: "+val.Value, "value", val.Value))
		}
	}
}

func obligation(working bool) Severity {
	if working {
		return SeverityWarning
	}
	return SeverityBlocking
}

func (v *validator) hasValue(p Path) bool {
	val, ok := v.doc.Values[p]
	return ok && val.Value != ""
}

func (v *validator) checkChildren(parent *Node, base Path, parentExport bool) {
	for _, c := range parent.Children {
		if c.ExportID {
			continue
		}
		export := parentExport && c.Export
		child := base.Child(c.Name)
		if c.IsRepeatable() {
			keys := v.doc.ItemKeys(child)
			// A repeatable field item with an empty value is not written to
			// the export: it does not count towards the minimum.
			filled := len(keys)
			if c.Kind == NodeField {
				filled = 0
				for _, k := range keys {
					if v.hasValue(Path(string(child) + "[" + k + "]")) {
						filled++
					}
				}
			}
			if filled < c.Min {
				v.issues = append(v.issues, Issue{Path: child, Code: CodeTooFewItems, Severity: obligation(!export),
					Message: fmt.Sprintf("at least %d item(s) (%d present)", c.Min, filled),
					Params:  Params("min", itoa(c.Min), "present", itoa(filled))})
			}
			// Same count as the minimum and as the export: empty field items are ignored.
			if c.Max >= 0 && filled > c.Max {
				v.add(blocking(child, CodeTooManyItems, fmt.Sprintf("at most %d items (%d present)", c.Max, filled),
					"max", itoa(c.Max), "present", itoa(filled)))
			}
			if c.Kind != NodeField {
				for _, k := range keys {
					v.checkChildren(c, Path(string(child)+"["+k+"]"), export)
				}
			}
			continue
		}
		if c.Kind == NodeField {
			if c.Min >= 1 && !v.hasValue(child) {
				v.issues = append(v.issues, Issue{Path: child, Code: CodeRequired, Severity: obligation(!export),
					Message: "required value"})
			}
			continue
		}
		if c.Min >= 1 || v.ancestors[child] {
			v.checkChildren(c, child, export)
		}
	}
}

// exists reports whether the concrete group path exists in the document: a
// value or item below it, or a mandatory group under an existing parent.
func (v *validator) exists(p Path) bool {
	if p == "" || v.ancestors[p] {
		return true
	}
	node := v.spec.NodeAt(p.Template())
	if node == nil || node.IsRepeatable() || node.Min < 1 {
		return false
	}
	return v.exists(p.Parent())
}

// expand resolves a relative template (segments, `[]` for every item) from a
// concrete base into concrete paths of existing items.
func (v *validator) expand(base Path, rest string) []Path {
	if rest == "" {
		return []Path{base}
	}
	seg, tail, _ := strings.Cut(rest, ".")
	name, repeated := strings.CutSuffix(seg, "[]")
	child := base.Child(name)
	if !repeated {
		return v.expand(child, tail)
	}
	var out []Path
	for _, k := range v.doc.ItemKeys(child) {
		out = append(out, v.expand(Path(string(child)+"["+k+"]"), tail)...)
	}
	return out
}

// strip resolves a template from base without enumeration: `[]` is dropped.
func strip(base Path, rest string) Path {
	return base.Child(strings.ReplaceAll(rest, "[]", ""))
}

type scopeInstance struct {
	template string
	concrete Path
}

func (s scopeInstance) rel(tmpl string) (Path, string, bool) {
	if s.template == "" || !(tmpl == s.template || strings.HasPrefix(tmpl, s.template+".")) {
		return "", tmpl, false
	}
	return s.concrete, strings.TrimPrefix(strings.TrimPrefix(tmpl, s.template), "."), true
}

// resolve returns the concrete paths of a rule template for this scope instance.
func (v *validator) resolve(s scopeInstance, tmpl string) []Path {
	if base, rest, ok := s.rel(tmpl); ok {
		return v.expand(base, rest)
	}
	return v.expand("", tmpl)
}

func (v *validator) resolveStatic(s scopeInstance, tmpl string) Path {
	if base, rest, ok := s.rel(tmpl); ok {
		return strip(base, rest)
	}
	return strip("", tmpl)
}

// presentPaths returns the concrete paths under tmpl that hold a value.
func (v *validator) presentPaths(s scopeInstance, tmpl string) []Path {
	var out []Path
	for _, p := range v.resolve(s, tmpl) {
		if v.valued[p] {
			out = append(out, p)
		}
	}
	return out
}

// condition evaluates a rule condition on the values of c.Path under the scope
// instance. A nil condition is always true. Semantics per operator:
//   - eq, in: some value is one of c.Values;
//   - not_in: no value is one of c.Values; an empty field counts as not in the
//     list, so "forbidden unless X is A" applies while X is empty (an optional X
//     may stay empty for good, and the rule must still hold);
//   - present, absent: the field has, or has not, a value.
func (v *validator) condition(s scopeInstance, c *Condition) bool {
	if c == nil {
		return true
	}
	var vals []string
	for _, p := range v.presentPaths(s, c.Path) {
		if val, ok := v.doc.Values[p]; ok {
			vals = append(vals, val.Value)
		} else {
			vals = append(vals, "")
		}
	}
	match := func() bool {
		return slices.ContainsFunc(vals, func(x string) bool { return slices.Contains(c.Values, x) })
	}
	switch c.Op {
	case opEq, opIn:
		return match()
	case opNotIn:
		return !match()
	case opPresent:
		return len(vals) > 0
	case opAbsent:
		return len(vals) == 0
	}
	return false
}

func knownOp(c *Condition) bool {
	return c == nil || KnownConditionOp(c.Op)
}

// KnownConditionOp reports whether op is a condition operator the engine evaluates.
func KnownConditionOp(op string) bool {
	return slices.Contains([]string{opEq, opIn, opNotIn, opPresent, opAbsent}, op)
}

// KnownRuleKind reports whether kind is a rule kind the engine evaluates.
func KnownRuleKind(kind string) bool {
	return slices.Contains([]string{ruleKindRequiredIf, ruleKindForbiddenIf, ruleKindExactlyOne, ruleKindAtMostOne}, kind)
}

func (v *validator) scopes(r Rule) []scopeInstance {
	if r.Scope == "" {
		return []scopeInstance{{}}
	}
	var out []scopeInstance
	for _, p := range v.expand("", r.Scope) {
		if v.exists(p) {
			out = append(out, scopeInstance{template: r.Scope, concrete: p})
		}
	}
	return out
}

func (v *validator) workTemplate(tmpl string) bool {
	n := v.spec.NodeAt(tmpl)
	return n != nil && !n.Export
}

func (v *validator) ruleIssue(r Rule, p Path, sev Severity) {
	msg := r.Description
	if msg == "" {
		msg = "rule not satisfied: " + r.ID
	}
	v.issues = append(v.issues, Issue{Path: p, Code: ruleCodePrefix + r.ID, Message: msg, Severity: sev, RuleID: r.ID})
}

func (v *validator) checkRule(r Rule) {
	// Checked once, whatever the scope instances: a rule scoped on an empty list
	// must not hide a spec error.
	if !knownOp(r.When) {
		v.add(blocking(Path(r.Scope), CodeUnknownRule, "unknown condition operator: "+r.When.Op+" ("+r.ID+")",
			"rule_id", r.ID, "op", r.When.Op))
		return
	}
	if !KnownRuleKind(r.Kind) {
		v.add(blocking(Path(r.Scope), CodeUnknownRule, "unknown rule kind: "+r.Kind+" ("+r.ID+")",
			"rule_id", r.ID, "kind", r.Kind))
		return
	}
	for _, s := range v.scopes(r) {
		if !v.condition(s, r.When) {
			continue
		}
		switch r.Kind {
		case ruleKindRequiredIf:
			for _, t := range r.Then {
				if len(v.presentPaths(s, t)) == 0 {
					sev := r.EffectiveSeverity()
					if v.workTemplate(t) {
						sev = SeverityWarning
					}
					v.ruleIssue(r, v.resolveStatic(s, t), sev)
				}
			}
		case ruleKindForbiddenIf:
			for _, t := range r.Then {
				for _, p := range v.presentPaths(s, t) {
					v.ruleIssue(r, p, r.EffectiveSeverity())
				}
			}
		case ruleKindExactlyOne, ruleKindAtMostOne:
			count := 0
			for _, a := range r.Among {
				count += len(v.presentPaths(s, a))
			}
			if count > 1 || (count == 0 && r.Kind == ruleKindExactlyOne) {
				p := s.concrete
				if r.Scope == "" && len(r.Among) > 0 {
					p = v.resolveStatic(s, r.Among[0])
				}
				sev := r.EffectiveSeverity()
				// An empty choice is only a warning when every alternative is a
				// working node: the order of Among must not change the outcome.
				if count == 0 && len(r.Among) > 0 && !slices.ContainsFunc(r.Among, func(a string) bool { return !v.workTemplate(a) }) {
					sev = SeverityWarning
				}
				v.ruleIssue(r, p, sev)
			}
		}
	}
}
