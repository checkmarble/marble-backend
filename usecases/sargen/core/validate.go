package core

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
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
	CodeCodeListUnavailable = "code_list_unavailable"
	ruleCodePrefix          = "rule:"
	ruleKindRequiredIf      = "required_if"
	ruleKindForbiddenIf     = "forbidden_if"
	ruleKindExactlyOne      = "exactly_one"
	ruleKindAtMostOne       = "at_most_one"
	opEq, opIn, opNotIn     = "eq", "in", "not_in"
	opPresent, opAbsent     = "present", "absent"
	dateLayout              = "2006-01-02"
)

var (
	decimalRe  = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)
	integerRe  = regexp.MustCompile(`^[0-9]+$`)
	patternMu  sync.Mutex
	patternMap = map[string]*regexp.Regexp{}
)

// anchoredPattern compiles `^(?:p)$` once per pattern. It returns nil when the
// pattern does not compile (the generator guarantees it does).
func anchoredPattern(p string) *regexp.Regexp {
	patternMu.Lock()
	defer patternMu.Unlock()
	if re, ok := patternMap[p]; ok {
		return re
	}
	re, err := regexp.Compile("^(?:" + p + ")$")
	if err != nil {
		re = nil
	}
	patternMap[p] = re
	return re
}

func blocking(path Path, code, msg string) *Issue {
	return &Issue{Path: path, Code: code, Message: msg, Severity: SeverityBlocking}
}

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
// accepted (it clears the field).
func ValidateValue(spec *Spec, path Path, raw string) *Issue {
	if _, err := ParsePath(string(path)); err != nil {
		return blocking(path, CodeUnknownPath, "chemin invalide : "+err.Error())
	}
	node := spec.NodeAt(path.Template())
	if node == nil || node.Template == "" {
		return blocking(path, CodeUnknownPath, "champ inconnu dans le format")
	}
	if node.ExportID {
		return blocking(path, CodeExportIDValue, "cet identifiant est généré à l'export : aucune valeur à saisir")
	}
	if node.Kind != NodeField {
		return blocking(path, CodeNotAField, "ce chemin désigne un groupe, pas un champ")
	}
	if raw == "" {
		return nil
	}
	return checkValue(spec, node, path, raw)
}

func checkValue(spec *Spec, node *Node, path Path, raw string) *Issue {
	switch node.Type {
	case TypeBoolean:
		if raw != "true" && raw != "false" {
			return blocking(path, CodeBadBoolean, "valeur attendue : true ou false")
		}
		return nil
	case TypeDate:
		if _, err := time.Parse(dateLayout, raw); err != nil {
			return blocking(path, CodeBadDate, "date invalide, format attendu AAAA-MM-JJ")
		}
		return nil
	case TypeInteger:
		if !integerRe.MatchString(raw) {
			return blocking(path, CodeBadInteger, "nombre entier attendu")
		}
		if node.MaxLength > 0 && len(raw) > node.MaxLength {
			return blocking(path, CodeMaxLength, fmt.Sprintf("%d chiffres au maximum", node.MaxLength))
		}
		return nil
	case TypeDecimal:
		return checkDecimal(node, path, raw)
	case TypeCode:
		return checkCode(spec, node, path, raw)
	case TypeRef:
		if !slices.Contains(node.RefTargets, Path(raw).Template()) {
			return blocking(path, CodeBadRef, "référence vers un élément de type non autorisé")
		}
		return nil
	}
	n := utf8.RuneCountInString(raw)
	if node.MinLength > 0 && n < node.MinLength {
		return blocking(path, CodeMinLength, fmt.Sprintf("%d caractères au minimum", node.MinLength))
	}
	if node.MaxLength > 0 && n > node.MaxLength {
		return blocking(path, CodeMaxLength, fmt.Sprintf("%d caractères au maximum (%d saisis)", node.MaxLength, n))
	}
	if node.Pattern != "" {
		if re := anchoredPattern(node.Pattern); re != nil && !re.MatchString(raw) {
			return blocking(path, CodePattern, "format de valeur invalide")
		}
	}
	return nil
}

func checkDecimal(node *Node, path Path, raw string) *Issue {
	if !decimalRe.MatchString(raw) {
		return blocking(path, CodeBadDecimal, "nombre décimal attendu (ex. 1234.56)")
	}
	digits := strings.TrimPrefix(raw, "-")
	intPart, frac, _ := strings.Cut(digits, ".")
	if node.FractionDigits > 0 && len(frac) > node.FractionDigits {
		return blocking(path, CodeBadDecimal, fmt.Sprintf("%d décimales au maximum", node.FractionDigits))
	}
	if node.MaxLength > 0 && len(intPart)+len(frac) > node.MaxLength {
		return blocking(path, CodeMaxLength, fmt.Sprintf("%d chiffres au maximum", node.MaxLength))
	}
	return nil
}

func checkCode(spec *Spec, node *Node, path Path, raw string) *Issue {
	list, ok := spec.CodeLists[node.CodeList]
	if !ok {
		return blocking(path, CodeUnknownCode, "liste de codes inconnue : "+node.CodeList)
	}
	if list.Unavailable {
		return &Issue{
			Path: path, Code: CodeCodeListUnavailable, Severity: SeverityWarning,
			Message: "liste de codes non publiée : valeur non vérifiée (" + node.CodeList + ")",
		}
	}
	if !slices.ContainsFunc(list.Codes, func(c Code) bool { return c.Value == raw }) {
		return blocking(path, CodeUnknownCode, "code inconnu dans la liste "+node.CodeList)
	}
	return nil
}

type validator struct {
	spec      *Spec
	doc       *Document
	ancestors map[Path]bool
	issues    []Issue
}

// Validate checks the whole document against the spec and its generic rules.
// Issues are ordered by path, then code.
func Validate(spec *Spec, doc *Document) []Issue {
	v := &validator{spec: spec, doc: doc, ancestors: map[Path]bool{}}
	v.indexAncestors()
	v.checkValues()
	if spec != nil && spec.Root != nil {
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
		}
	}
	for container, keys := range v.doc.Items {
		for _, k := range keys {
			v.addAncestors(Path(string(container) + "[" + k + "]"))
		}
	}
}

func (v *validator) itemExists(item Path) bool {
	s := string(item)
	i := strings.LastIndex(s, "[")
	if i < 0 || !strings.HasSuffix(s, "]") {
		return false
	}
	key := s[i+1 : len(s)-1]
	return slices.Contains(v.doc.Items[Path(s[:i])], key)
}

// itemsPresent checks every keyed segment of the path points to an existing item.
func (v *validator) itemsPresent(p Path) bool {
	segs := strings.Split(string(p), ".")
	for i, seg := range segs {
		if strings.HasSuffix(seg, "]") && !v.itemExists(Path(strings.Join(segs[:i+1], "."))) {
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
			v.add(blocking(p, CodeUnknownItem, "l'élément contenant cette valeur n'existe pas"))
			continue
		}
		issue := ValidateValue(v.spec, p, val.Value)
		if issue != nil {
			v.add(issue)
			continue
		}
		if node := v.spec.NodeAt(p.Template()); node != nil && node.Type == TypeRef && !v.itemExists(Path(val.Value)) {
			v.add(blocking(p, CodeDanglingRef, "la référence vise un élément absent : "+val.Value))
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
		if c.repeatable() {
			keys := v.doc.Items[child]
			if len(keys) < c.Min {
				v.issues = append(v.issues, Issue{Path: child, Code: CodeTooFewItems, Severity: obligation(!export),
					Message: fmt.Sprintf("%d élément(s) au minimum (%d présent(s))", c.Min, len(keys))})
			}
			if c.Max >= 0 && len(keys) > c.Max {
				v.add(blocking(child, CodeTooManyItems, fmt.Sprintf("%d éléments au maximum (%d présents)", c.Max, len(keys))))
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
					Message: "valeur obligatoire"})
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
	if node == nil || node.repeatable() || node.Min < 1 {
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
	for _, k := range v.doc.Items[child] {
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
		if v.ancestors[p] {
			out = append(out, p)
		}
	}
	return out
}

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
		msg = "règle non respectée : " + r.ID
	}
	v.issues = append(v.issues, Issue{Path: p, Code: ruleCodePrefix + r.ID, Message: msg, Severity: sev, RuleID: r.ID})
}

func (v *validator) checkRule(r Rule) {
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
				if count == 0 && len(r.Among) > 0 && v.workTemplate(r.Among[0]) {
					sev = SeverityWarning
				}
				v.ruleIssue(r, p, sev)
			}
		}
	}
}
