package core

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// Issue codes produced by ApplyFills (in addition to those of ValidateValue).
const (
	CodeOutOfScope           = "out_of_scope"
	CodeMissingSource        = "missing_source"
	CodeMissingJustification = "missing_justification"
	CodeNotWritableByLLM     = "not_writable_by_llm"
	CodeSourceOutOfScope     = "source_out_of_scope"
	CodeBadProvisionalKey    = "bad_provisional_key"
	CodeEmptyNewItem         = "empty_new_item"
	CodeMissingOrigin        = "missing_origin"
	CodeUnknownOrigin        = "unknown_origin"
	CodeBadConfidence        = "bad_confidence"
	CodeNotFound             = "not_found"
	CodeOriginNotAllowed     = "origin_not_allowed"
)

// OverwritePolicy decides what happens when a fill targets a non-empty value.
type OverwritePolicy string

const (
	// OverwriteNever (default): any existing non-empty value is a conflict.
	OverwriteNever OverwritePolicy = ""
	// OverwriteNonManual overwrites, except values written by the analyst
	// (OriginManual) and any Reviewed value.
	OverwriteNonManual OverwritePolicy = "non_manual"
	// OverwriteAll overwrites anything (manual edit by the analyst).
	OverwriteAll OverwritePolicy = "all"
)

// Fill is one requested write of a leaf of the document.
type Fill struct {
	Path          Path         `json:"path"`
	Value         string       `json:"value"`
	Origin        Origin       `json:"origin"`
	Sources       []SourceRef  `json:"sources"`
	RuleIDs       []string     `json:"rule_ids"` //nolint:tagliatelle
	Justification string       `json:"justification"`
	Confidence    Confidence   `json:"confidence"`
	Computation   *Computation `json:"computation,omitempty"`
	TaskID        *uuid.UUID   `json:"task_id,omitempty"`
	// Reviewed is stored as is, whatever the origin: typing a value is not
	// reviewing it. A caller whose write is a review (an option chosen in a
	// question) marks it with MarkReviewed.
	Reviewed bool `json:"reviewed"`
}

type ApplyOptions struct {
	// AllowedPrefixes restricts writable paths (by segments); an empty list
	// means "no restriction" (plan semantics), not "nothing allowed". An empty
	// entry matches no path, so a list holding only empty entries allows
	// nothing (a task with neither a target nor allowed paths writes nothing).
	// An item prefix (`a.b[k]`) allows everything below it, including new
	// provisional items; a prefix naming a repeatable container without key
	// (`a.b`) allows its existing items and new items `a.b[+x]`.
	AllowedPrefixes []Path
	Overwrite       OverwritePolicy
	// SkipMaxItems disables the maximum item count of repeatable containers at
	// write time (zero value: enforced). Compute sets it so that an excess
	// surfaces as the validator's blocking too_many_items instead of being
	// silently dropped. Repeatable field containers count filled items only.
	SkipMaxItems bool
	// AllowedSources returns the objects a fill on target may cite; nil = no
	// check. Target is the path where already-existing and already-created
	// items carry their real keys; items still to be created stay `[+x]`.
	AllowedSources func(target Path) []ObjectRef
}

type Rejected struct {
	Fill  Fill
	Issue Issue
}

// Conflict is a fill refused because it would overwrite a protected value.
type Conflict struct {
	Path   Path
	Before Value
	After  Fill
}

type ApplyResult struct {
	// Applied lists the real paths written or cleared, in input order.
	Applied []Path
	// Refreshed lists the paths whose stored computed value was kept as is but
	// whose computation (the inputs it cites) was updated, because a recompute
	// gave the same value from different inputs. A refresh is not an applied
	// write (no todo, question or conflict follows from it) but it modifies the
	// document, so a caller that persists the document must treat it as a
	// change.
	Refreshed []Path
	Rejected  []Rejected
	Conflicts []Conflict
	// Resolved maps each provisional item prefix as written in the fills
	// (e.g. `personnes.pp[+a]`, `personnes.pp[+a].etatCivil.prenom[+x]`) to
	// the real item path it was created as.
	Resolved map[Path]Path
}

// Rewrite replaces the provisional prefixes of p that were resolved by their
// real item path (longest prefix first). Other paths are returned unchanged.
func (r ApplyResult) Rewrite(p Path) Path {
	best := Path("")
	for k := range r.Resolved {
		if (p == k || strings.HasPrefix(string(p), string(k)+".")) && len(k) > len(best) {
			best = k
		}
	}
	if best == "" {
		return p
	}
	return Path(string(r.Resolved[best]) + strings.TrimPrefix(string(p), string(best)))
}

// MarkReviewed sets Reviewed on the existing values at paths. An empty or
// unknown path yields unknown_path, a known path without value not_found.
func MarkReviewed(spec *Spec, doc *Document, paths []Path) []Issue {
	var issues []Issue
	for _, p := range paths {
		if p == "" {
			issues = append(issues, *blocking(p, CodeUnknownPath, "empty path"))
			continue
		}
		if _, err := ParsePath(string(p)); err != nil {
			issues = append(issues, *blocking(p, CodeUnknownPath, "invalid path: "+err.Error()))
			continue
		}
		if n := spec.NodeAt(p.Template()); n == nil || n.Template == "" {
			issues = append(issues, *blocking(p, CodeUnknownPath, "unknown field in the format"))
			continue
		}
		v, ok := doc.Values[p]
		if !ok || v.Value == "" {
			issues = append(issues, *blocking(p, CodeNotFound, "no value at this path"))
			continue
		}
		v.Reviewed = true
		doc.Values[p] = v
	}
	return issues
}

type pendingSeg struct {
	index int    // segment index in the path
	key   string // literal prefix up to and including this segment, e.g. `a.b[+x]`
}

// ApplyFills is the single write path of a declaration document. Each fill is
// checked in order: (1) provisional keys, (2) path (known leaf, existing
// parent items, allowed prefix), (3) origin and nature rules, (4) allowed
// sources, (5) reference resolution, value validation, then overwrite
// policy. A refused fill has no effect; a provisional item is created only
// once its fill passed every check. Fills are independent: one refused fill
// does not undo the others of the same call.
//
// Writing the value already stored is a no-op (neither applied nor a
// conflict); reviewing a value is MarkReviewed, never a rewrite. An empty value
// clears the field.
//
// A computed field stays tied to its inputs. The analyst may correct it by
// hand: the correction keeps the computation, with its inputs and the value
// it gave (Computation.Result). A recompute giving that same value from the
// same inputs keeps the correction; another value, or other inputs, replaces
// it, unreviewed (OverwriteNonManual). A manual value typed while the field had
// no computed value is replaced by the first computed value.
func ApplyFills(spec *Spec, doc *Document, fills []Fill, opts ApplyOptions) ApplyResult {
	var res ApplyResult
	resolved := map[string]Path{} // provisional literal prefix -> real item path

	for _, f := range fills {
		real, pend, issue := walkPath(spec, doc, f.Path, opts, resolved, false)
		if issue == nil {
			issue = checkFillMeta(f)
		}
		if issue == nil {
			issue = checkNature(spec, f)
		}
		if issue == nil {
			issue = checkSources(f, real, opts)
		}
		if issue == nil {
			f.Value, issue = resolveRef(spec, doc, f, resolved)
		}
		if issue == nil {
			issue = checkFillValue(spec, f)
		}
		if issue == nil && len(pend) > 0 && f.Value == "" {
			issue = blocking(f.Path, CodeEmptyNewItem, "cannot clear a value of an item that does not exist yet")
		}
		if issue != nil {
			res.Rejected = append(res.Rejected, Rejected{Fill: f, Issue: *issue})
			continue
		}
		if len(pend) == 0 {
			before, exists := doc.Values[real]
			if exists && before.Value == "" {
				exists = false
			}
			if !exists && f.Value == "" {
				continue
			}
			computedField := spec.NodeAt(f.Path.Template()).EffectiveNature() == NatureComputed
			if exists {
				switch {
				case before.Value == f.Value:
					if refreshComputation(doc, real, before, f) {
						res.Refreshed = append(res.Refreshed, real)
					}
					continue
				case computedField && keepsCorrection(before, f):
					continue
				case protected(before, f, opts.Overwrite, computedField):
					res.Conflicts = append(res.Conflicts, Conflict{Path: real, Before: before, After: f})
					continue
				}
				if computedField && f.Origin == OriginManual {
					f.Computation = correctedComputation(before)
				}
			}
		} else {
			real, _, _ = walkPath(spec, doc, f.Path, opts, resolved, true)
		}
		write(doc, real, f)
		res.Applied = append(res.Applied, real)
	}
	if len(resolved) > 0 {
		res.Resolved = make(map[Path]Path, len(resolved))
		for k, v := range resolved {
			res.Resolved[Path(k)] = v
		}
	}
	return res
}

// refreshComputation keeps the stored computed value but replaces its
// computation when a recompute gives the same value from other inputs, so the
// provenance never cites stale inputs. Value, origin, reviewed flag and every
// other provenance field are untouched; values of other origins are never
// concerned. It reports whether the stored computation changed.
func refreshComputation(doc *Document, p Path, before Value, f Fill) bool {
	if f.Origin != OriginComputed || before.Origin != OriginComputed || f.Computation == nil {
		return false
	}
	if before.Computation != nil && before.Computation.Name == f.Computation.Name &&
		slices.Equal(before.Computation.Inputs, f.Computation.Inputs) {
		return false
	}
	c := *f.Computation
	c.Inputs = slices.Clone(c.Inputs)
	before.Computation = &c
	doc.Values[p] = before
	return true
}

// protected reports whether an existing non-empty value must not be replaced.
// On a computed field, a computed fill may replace a stored computed value even
// if reviewed, and a manual correction whose computation now gives another
// value (keepsCorrection handled the same value); the new value is written
// unreviewed.
func protected(v Value, f Fill, policy OverwritePolicy, computedField bool) bool {
	switch policy {
	case OverwriteAll:
		return false
	case OverwriteNonManual:
		if f.Origin == OriginComputed && (v.Origin == OriginComputed || computedField) {
			return false
		}
		return v.Reviewed || v.Origin == OriginManual
	default:
		return true
	}
}

// keepsCorrection reports a recompute of a computed field that the analyst
// corrected by hand, giving the value the computation gave at correction time
// from the same inputs (the same paths, in any order): the correction was made
// for this state of the data and stays. Another value, or the same value from
// other inputs (a transaction added or removed), replaces it.
func keepsCorrection(before Value, f Fill) bool {
	if f.Origin != OriginComputed || before.Origin != OriginManual || before.Computation == nil ||
		before.Computation.Result != f.Value {
		return false
	}
	var inputs []Path
	if f.Computation != nil {
		inputs = f.Computation.Inputs
	}
	return sameInputs(before.Computation.Inputs, inputs)
}

// sameInputs compares two lists of input paths as sets.
func sameInputs(a, b []Path) bool {
	if len(a) != len(b) {
		return false
	}
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// correctedComputation is the computation kept on a manual correction of a
// computed field: the stored computation with the value it gave, or the
// computation of an earlier correction. Nil when the field had no computed
// value (a value typed while nothing could be computed).
func correctedComputation(before Value) *Computation {
	if before.Computation == nil {
		return nil
	}
	c := *before.Computation
	c.Inputs = slices.Clone(c.Inputs)
	if before.Origin == OriginComputed {
		c.Result = before.Value
	}
	return &c
}

func write(doc *Document, p Path, f Fill) {
	if f.Value == "" {
		delete(doc.Values, p)
		return
	}
	v := Value{
		Value:         f.Value,
		Origin:        f.Origin,
		Sources:       slices.Clone(f.Sources),
		RuleIDs:       slices.Clone(f.RuleIDs),
		Justification: f.Justification,
		Confidence:    f.Confidence,
		Reviewed:      f.Reviewed,
		TaskID:        f.TaskID,
	}
	if f.Computation != nil {
		c := *f.Computation
		c.Inputs = slices.Clone(c.Inputs)
		v.Computation = &c
	}
	doc.Values[p] = v
}

// PathAllowed reports whether p lies within one of the prefixes (by segments,
// an item key `[k]` on the prefix's last segment counting as within it). No
// prefix allows nothing here, unlike ApplyOptions.AllowedPrefixes where an
// empty list means "no restriction".
func PathAllowed(p Path, prefixes []Path) bool {
	return slices.ContainsFunc(prefixes, func(pre Path) bool { return prefixAllows(p, pre) })
}

func prefixAllows(real, prefix Path) bool {
	if prefix == "" {
		return false
	}
	return real.HasPrefix(prefix) || strings.HasPrefix(string(real), string(prefix)+"[")
}

// walkPath is the single segment walker. Resolved provisional segments are
// replaced by their real item path. In check mode (create=false) it validates
// the path (known leaf, existing parents, allowed prefix) and returns the
// pending provisional segments, which keep their `[+x]` key in the returned
// path. In create mode it creates the pending items and returns the real path.
func walkPath(spec *Spec, doc *Document, p Path, opts ApplyOptions, resolved map[string]Path, create bool) (Path, []pendingSeg, *Issue) {
	if !create {
		if _, err := ParsePath(string(p)); err != nil {
			return "", nil, blocking(p, CodeUnknownPath, "invalid path: "+err.Error())
		}
		node := spec.NodeAt(p.Template())
		if node == nil || node.Template == "" {
			return "", nil, blocking(p, CodeUnknownPath, "unknown field in the format")
		}
		if node.ExportID {
			return "", nil, blocking(p, CodeExportIDValue, "this identifier is generated at export: no value to enter")
		}
		if node.Kind != NodeField {
			return "", nil, blocking(p, CodeNotAField, "this path designates a group, not a field")
		}
	}

	segs := strings.Split(string(p), ".")
	var real Path
	var pend []pendingSeg
	var tmpl []string
	// Each keyed segment is one of three cases: a real key (`[pp-1]`), checked
	// against the existing items; a provisional key (`[+a]`) already resolved
	// in this call, replaced by the item it created; or a provisional key still
	// to create, recorded as pending (check mode) or created (create mode).
	for i, seg := range segs {
		name, key := seg, ""
		if j := strings.Index(seg, "["); j >= 0 {
			name, key = seg[:j], seg[j+1:len(seg)-1]
		}
		if key == "" {
			tmpl = append(tmpl, name)
			real = real.Child(name)
			continue
		}
		tmpl = append(tmpl, name+"[]")
		literal := strings.Join(segs[:i+1], ".")
		container := real.Child(name)
		if !strings.HasPrefix(key, "+") {
			if !create && (len(pend) > 0 || !slices.Contains(doc.ItemKeys(container), key)) {
				return "", nil, blocking(p, CodeUnknownItem, "the item "+seg+" does not exist", "item", seg)
			}
			real = Path(string(container) + "[" + key + "]")
			continue
		}
		if r, ok := resolved[literal]; ok {
			real = r
			continue
		}
		if create {
			real = doc.AddItem(container, name)
			resolved[literal] = real
			continue
		}
		n := spec.NodeAt(strings.Join(tmpl, "."))
		if n == nil || !n.IsRepeatable() {
			return "", nil, blocking(p, CodeBadProvisionalKey, "provisional key on a node that is not repeatable")
		}
		if !opts.SkipMaxItems && n.Max > 0 && countItems(doc, n, container) >= n.Max {
			return "", nil, blocking(p, CodeTooManyItems, fmt.Sprintf("at most %d items: the maximum is reached", n.Max), "max", itoa(n.Max))
		}
		pend = append(pend, pendingSeg{index: i, key: literal})
		real = Path(string(container) + "[" + key + "]")
	}
	if !create && len(opts.AllowedPrefixes) > 0 {
		if !PathAllowed(real, opts.AllowedPrefixes) {
			return "", nil, blocking(p, CodeOutOfScope, "path outside the part assigned to this task")
		}
	}
	return real, pend, nil
}

// countItems counts the items of a container as the validator does: all items
// of a group container, only filled ones for a field container.
func countItems(doc *Document, n *Node, container Path) int {
	keys := doc.ItemKeys(container)
	if n.Kind != NodeField {
		return len(keys)
	}
	c := 0
	for _, k := range keys {
		if v, ok := doc.Values[Path(string(container)+"["+k+"]")]; ok && v.Value != "" {
			c++
		}
	}
	return c
}

func checkFillMeta(f Fill) *Issue {
	if f.Origin == "" {
		return blocking(f.Path, CodeMissingOrigin, "the origin of the value is required")
	}
	switch f.Origin {
	case OriginLLM, OriginOrgSetting, OriginComputed, OriginManual:
	default:
		return blocking(f.Path, CodeUnknownOrigin, "unknown origin of the value", "origin", string(f.Origin))
	}
	switch f.Confidence {
	case ConfidenceNone, ConfidenceLow, ConfidenceMedium, ConfidenceHigh:
	default:
		return blocking(f.Path, CodeBadConfidence, "unknown confidence level", "confidence", string(f.Confidence))
	}
	return nil
}

// resolveRef rewrites a reference value naming a provisional item created
// earlier in the same call, and checks that a reference targets an existing
// item. Free text of a RefOrText field is left untouched.
func resolveRef(spec *Spec, doc *Document, f Fill, resolved map[string]Path) (string, *Issue) {
	node := spec.NodeAt(f.Path.Template())
	if f.Value == "" || node == nil {
		return f.Value, nil
	}
	isRef := node.Type == TypeRef || node.IsItemRef(f.Value)
	if !isRef {
		return f.Value, nil
	}
	if _, err := ParsePath(f.Value); err != nil || !strings.Contains(f.Value, "[") {
		return f.Value, nil // left to ValidateValue (bad_ref)
	}
	segs := strings.Split(f.Value, ".")
	var real Path
	for i, seg := range segs {
		j := strings.Index(seg, "[")
		if j < 0 {
			real = real.Child(seg)
			continue
		}
		name, key := seg[:j], seg[j+1:len(seg)-1]
		container := real.Child(name)
		if strings.HasPrefix(key, "+") {
			r, ok := resolved[strings.Join(segs[:i+1], ".")]
			if !ok {
				return f.Value, blocking(f.Path, CodeBadRef, "reference to an unknown item", "value", f.Value)
			}
			real = r
			continue
		}
		if !slices.Contains(doc.ItemKeys(container), key) {
			return f.Value, blocking(f.Path, CodeDanglingRef, "the reference targets a missing item: "+f.Value, "value", f.Value)
		}
		real = Path(string(container) + "[" + key + "]")
	}
	return string(real), nil
}

// checkNature checks which origins may write which fields: organization
// settings only through OriginOrgSetting or by the analyst, OriginOrgSetting
// only on settings, and the LLM output rules.
func checkNature(spec *Spec, f Fill) *Issue {
	nature := spec.NodeAt(f.Path.Template()).EffectiveNature()
	if f.Origin != OriginLLM {
		settingOrigin := f.Origin == OriginOrgSetting || f.Origin == OriginManual
		if (nature == NatureSetting && !settingOrigin) || (f.Origin == OriginOrgSetting && nature != NatureSetting) {
			return blocking(f.Path, CodeOriginNotAllowed, "this origin cannot write this field", "origin", string(f.Origin))
		}
		return nil
	}
	switch nature {
	case NatureSetting, NatureComputed, NatureAnalyst:
		return blocking(f.Path, CodeNotWritableByLLM, "this field cannot be filled by the AI")
	case NatureCopied:
		if len(f.Sources) == 0 {
			return blocking(f.Path, CodeMissingSource, "a copied value must cite at least one source")
		}
	case NatureJudgment:
		if strings.TrimSpace(f.Justification) == "" || f.Confidence == ConfidenceNone {
			return blocking(f.Path, CodeMissingJustification, "a judgment needs a justification and a confidence level")
		}
	}
	return nil
}

func checkSources(f Fill, target Path, opts ApplyOptions) *Issue {
	if opts.AllowedSources == nil {
		return nil
	}
	allowed := opts.AllowedSources(target)
	for _, s := range f.Sources {
		if !slices.Contains(allowed, s.Object) {
			return blocking(f.Path, CodeSourceOutOfScope, "source outside the scope allowed for this field")
		}
	}
	return nil
}

func checkFillValue(spec *Spec, f Fill) *Issue {
	if issue := ValidateValue(spec, f.Path, f.Value); issue.Blocks() {
		return issue
	}
	return nil
}
