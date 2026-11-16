package core

import (
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
	CodeBadConfidence        = "bad_confidence"
	CodeNotFound             = "not_found"
)

// OverwritePolicy decides what happens when a fill targets a non-empty value.
type OverwritePolicy string

const (
	// OverwriteNever (default): any existing non-empty value is a conflict.
	OverwriteNever OverwritePolicy = ""
	// OverwriteNonManual overwrites, except values written by the analyst
	// (OriginManual, OriginAnalyst) and any Reviewed value.
	OverwriteNonManual OverwritePolicy = "non_manual"
	// OverwriteAll overwrites anything (manual edit by the analyst).
	OverwriteAll OverwritePolicy = "all"
)

// Fill is one requested write of a leaf of the document.
type Fill struct {
	Path          Path
	Value         string
	Origin        Origin
	Sources       []SourceRef
	RuleIDs       []string
	Justification string
	Confidence    Confidence
	Computation   *Computation
	TaskID        *uuid.UUID
	// Reviewed is stored as is (analyst edits may be reviewed at once).
	Reviewed bool
}

type ApplyOptions struct {
	// AllowedPrefixes restricts writable paths (by segments); empty list = no
	// restriction, empty entries are ignored. An item prefix (`a.b[k]`) allows
	// everything below it, including new provisional items; a prefix naming a
	// repeatable container without key (`a.b`) allows its existing items and
	// new items `a.b[+x]`.
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
	Applied   []Path
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
			issues = append(issues, *blocking(p, CodeUnknownPath, "chemin vide"))
			continue
		}
		if _, err := ParsePath(string(p)); err != nil {
			issues = append(issues, *blocking(p, CodeUnknownPath, "chemin invalide : "+err.Error()))
			continue
		}
		if n := spec.NodeAt(p.Template()); n == nil || n.Template == "" {
			issues = append(issues, *blocking(p, CodeUnknownPath, "champ inconnu dans le format"))
			continue
		}
		v, ok := doc.Values[p]
		if !ok || v.Value == "" {
			issues = append(issues, *blocking(p, CodeNotFound, "aucune valeur à ce chemin"))
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
// parent items, allowed prefix), (3) nature rules for LLM output, (4) allowed
// sources, (5) reference resolution, value validation, then overwrite
// policy. A refused fill has no effect; a provisional item is created only
// once its fill passed every check.
//
// Writing the value already stored is a no-op (neither applied nor a
// conflict), except an analyst confirmation (OverwriteAll, or a manual or
// analyst fill over a value of another origin) which rewrites the metadata
// and counts as applied. An empty value clears the field.
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
			issue = blocking(f.Path, CodeEmptyNewItem, "impossible d'effacer une valeur d'un élément qui n'existe pas encore")
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
			if exists {
				confirm := before.Value == f.Value && isAnalystOrigin(f.Origin) &&
					(opts.Overwrite == OverwriteAll || before.Origin != f.Origin)
				switch {
				case confirm:
				case before.Value == f.Value:
					continue
				case protected(before, f, opts.Overwrite):
					res.Conflicts = append(res.Conflicts, Conflict{Path: real, Before: before, After: f})
					continue
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

func isAnalystOrigin(o Origin) bool { return o == OriginManual || o == OriginAnalyst }

// protected reports whether an existing non-empty value must not be replaced.
// A computed fill may recompute a stored computed value even if reviewed (the
// new value is written unreviewed).
func protected(v Value, f Fill, policy OverwritePolicy) bool {
	switch policy {
	case OverwriteAll:
		return false
	case OverwriteNonManual:
		if f.Origin == OriginComputed && v.Origin == OriginComputed {
			return false
		}
		return v.Reviewed || isAnalystOrigin(v.Origin)
	default:
		return true
	}
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
			return "", nil, blocking(p, CodeUnknownPath, "chemin invalide : "+err.Error())
		}
		node := spec.NodeAt(p.Template())
		if node == nil || node.Template == "" {
			return "", nil, blocking(p, CodeUnknownPath, "champ inconnu dans le format")
		}
		if node.ExportID {
			return "", nil, blocking(p, CodeExportIDValue, "cet identifiant est généré à l'export : aucune valeur à saisir")
		}
		if node.Kind != NodeField {
			return "", nil, blocking(p, CodeNotAField, "ce chemin désigne un groupe, pas un champ")
		}
	}

	segs := strings.Split(string(p), ".")
	var real Path
	var pend []pendingSeg
	var tmpl []string
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
			if !create && (len(pend) > 0 || !slices.Contains(doc.Items[container], key)) {
				return "", nil, blocking(p, CodeUnknownItem, "l'élément "+seg+" n'existe pas")
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
			return "", nil, blocking(p, CodeBadProvisionalKey, "clé provisoire sur un bloc non répétable")
		}
		if !opts.SkipMaxItems && n.Max > 0 && countItems(doc, n, container) >= n.Max {
			return "", nil, blocking(p, CodeTooManyItems, "nombre maximal d'éléments atteint")
		}
		pend = append(pend, pendingSeg{index: i, key: literal})
		real = Path(string(container) + "[" + key + "]")
	}
	if !create && len(opts.AllowedPrefixes) > 0 {
		if !slices.ContainsFunc(opts.AllowedPrefixes, func(pre Path) bool { return prefixAllows(real, pre) }) {
			return "", nil, blocking(p, CodeOutOfScope, "chemin hors de la partie confiée à cette tâche")
		}
	}
	return real, pend, nil
}

// countItems counts the items of a container as the validator does: all items
// of a group container, only filled ones for a field container.
func countItems(doc *Document, n *Node, container Path) int {
	keys := doc.Items[container]
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
		return blocking(f.Path, CodeMissingOrigin, "l'origine de la valeur est obligatoire")
	}
	switch f.Confidence {
	case ConfidenceNone, ConfidenceLow, ConfidenceMedium, ConfidenceHigh:
	default:
		return blocking(f.Path, CodeBadConfidence, "niveau de confiance inconnu")
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
				return f.Value, blocking(f.Path, CodeBadRef, "référence à un élément inconnu")
			}
			real = r
			continue
		}
		if !slices.Contains(doc.Items[container], key) {
			return f.Value, blocking(f.Path, CodeDanglingRef, "la référence vise un élément absent : "+f.Value)
		}
		real = Path(string(container) + "[" + key + "]")
	}
	return string(real), nil
}

func checkNature(spec *Spec, f Fill) *Issue {
	if f.Origin != OriginLLM {
		return nil
	}
	node := spec.NodeAt(f.Path.Template())
	switch node.EffectiveNature() {
	case NatureSetting, NatureComputed, NatureAnalyst:
		return blocking(f.Path, CodeNotWritableByLLM, "ce champ ne peut pas être rempli par l'IA")
	case NatureCopied:
		if len(f.Sources) == 0 {
			return blocking(f.Path, CodeMissingSource, "une valeur recopiée doit citer au moins une source")
		}
	case NatureJudgment:
		if strings.TrimSpace(f.Justification) == "" || f.Confidence == ConfidenceNone {
			return blocking(f.Path, CodeMissingJustification, "un jugement doit comporter une justification et un niveau de confiance")
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
			return blocking(f.Path, CodeSourceOutOfScope, "source hors du périmètre autorisé pour ce champ")
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
