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
}

type ApplyOptions struct {
	// AllowedPrefixes restricts writable paths (by segments); empty = no restriction.
	AllowedPrefixes []Path
	Overwrite       OverwritePolicy
	// AllowedSources returns the objects a fill on target may cite; nil = no check.
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
}

type pendingSeg struct {
	index int    // segment index in the path
	key   string // literal prefix up to and including this segment, e.g. `a.b[+x]`
}

type createdItem struct {
	item      Path
	container Path
	nextKey   int
	hadNext   bool
}

// ApplyFills is the single write path of a declaration document. Each fill is
// checked in order: (1) provisional keys, (2) path (known leaf, existing
// parent items, allowed prefix), (3) nature rules for LLM output, (4) allowed
// sources, (5) value validation then overwrite policy. A refused fill has no
// effect; a provisional item is created only once its fill passed every
// check, and removed again if none of its fills ends up applied.
//
// Writing the value already stored is a no-op: it is neither applied nor a
// conflict (metadata of the stored value is kept). An empty value clears the
// field.
func ApplyFills(spec *Spec, doc *Document, fills []Fill, opts ApplyOptions) ApplyResult {
	var res ApplyResult
	resolved := map[string]Path{} // provisional literal prefix -> real item path
	var created []createdItem
	usedItems := map[Path]bool{}

	for _, f := range fills {
		real, pend, issue := checkPath(spec, doc, f, opts, resolved)
		if issue == nil {
			issue = checkNature(spec, f)
		}
		if issue == nil {
			issue = checkSources(f, opts)
		}
		if issue == nil {
			issue = checkFillValue(spec, f)
		}
		if issue != nil {
			res.Rejected = append(res.Rejected, Rejected{Fill: f, Issue: *issue})
			continue
		}
		if len(pend) > 0 && f.Value == "" {
			res.Rejected = append(res.Rejected, Rejected{Fill: f, Issue: *blocking(f.Path, CodeEmptyNewItem,
				"impossible d'effacer une valeur d'un élément qui n'existe pas encore")})
			continue
		}
		if len(pend) == 0 {
			before, exists := doc.Values[real]
			if exists && before.Value == "" {
				exists = false
			}
			if exists && before.Value == f.Value {
				continue
			}
			if exists && protected(before, opts.Overwrite) {
				res.Conflicts = append(res.Conflicts, Conflict{Path: real, Before: before, After: f})
				continue
			}
			if !exists && f.Value == "" {
				continue
			}
		} else {
			real = createItems(doc, f.Path, pend, resolved, &created)
		}
		write(doc, real, f)
		res.Applied = append(res.Applied, real)
		markUsed(usedItems, real)
	}

	// No orphan items: remove created items none of whose fills was applied.
	for i := len(created) - 1; i >= 0; i-- {
		c := created[i]
		if usedItems[c.item] {
			continue
		}
		doc.RemoveItem(c.item)
		if c.hadNext {
			doc.NextKey[c.container] = c.nextKey
		} else {
			delete(doc.NextKey, c.container)
		}
	}
	return res
}

func markUsed(used map[Path]bool, p Path) {
	s := string(p)
	for {
		i := strings.LastIndex(s, "[")
		if i < 0 {
			return
		}
		used[Path(s[:strings.Index(s[i:], "]")+i+1])] = true
		s = s[:i]
	}
}

// protected reports whether an existing non-empty value must not be replaced.
func protected(v Value, policy OverwritePolicy) bool {
	switch policy {
	case OverwriteAll:
		return false
	case OverwriteNonManual:
		return v.Reviewed || v.Origin == OriginManual || v.Origin == OriginAnalyst
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
		Reviewed:      false,
		TaskID:        f.TaskID,
	}
	if f.Computation != nil {
		c := *f.Computation
		c.Inputs = slices.Clone(c.Inputs)
		v.Computation = &c
	}
	doc.Values[p] = v
}

// checkPath runs step 2 (and the provisional-key analysis of step 1). It
// returns the real path when the path has no pending provisional key.
func checkPath(spec *Spec, doc *Document, f Fill, opts ApplyOptions, resolved map[string]Path) (Path, []pendingSeg, *Issue) {
	p := f.Path
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

	segs := strings.Split(string(p), ".")
	var real Path
	var pend []pendingSeg
	var tmpl []string
	for i, seg := range segs {
		name, key := seg, ""
		if j := strings.Index(seg, "["); j >= 0 {
			name, key = seg[:j], seg[j+1:len(seg)-1]
		}
		tmpl = append(tmpl, name)
		if key == "" {
			real = real.Child(name)
			continue
		}
		tmpl[len(tmpl)-1] = name + "[]"
		literal := strings.Join(segs[:i+1], ".")
		container := real.Child(name)
		switch {
		case strings.HasPrefix(key, "+"):
			if r, ok := resolved[literal]; ok && len(pend) == 0 {
				real = r
				continue
			}
			n := spec.NodeAt(strings.Join(tmpl, "."))
			if n == nil || !n.IsRepeatable() {
				return "", nil, blocking(p, CodeBadProvisionalKey, "clé provisoire sur un bloc non répétable")
			}
			if n.Max > 0 && len(doc.Items[container]) >= n.Max {
				return "", nil, blocking(p, CodeTooManyItems, "nombre maximal d'éléments atteint")
			}
			pend = append(pend, pendingSeg{index: i, key: literal})
			real = Path(string(container) + "[" + key + "]")
		default:
			if len(pend) > 0 || !slices.Contains(doc.Items[container], key) {
				return "", nil, blocking(p, CodeUnknownItem, "l'élément "+seg+" n'existe pas")
			}
			real = Path(string(container) + "[" + key + "]")
		}
	}
	if len(opts.AllowedPrefixes) > 0 {
		ok := slices.ContainsFunc(opts.AllowedPrefixes, func(pre Path) bool { return real.HasPrefix(pre) })
		if !ok {
			return "", nil, blocking(p, CodeOutOfScope, "chemin hors de la partie confiée à cette tâche")
		}
	}
	return real, pend, nil
}

// createItems creates the items of the pending segments (outer to inner) and
// returns the real path of the fill.
func createItems(doc *Document, orig Path, pend []pendingSeg, resolved map[string]Path, created *[]createdItem) Path {
	segs := strings.Split(string(orig), ".")
	var real Path
	pi := 0
	for i, seg := range segs {
		if pi < len(pend) && pend[pi].index == i {
			name := seg[:strings.Index(seg, "[")]
			container := real.Child(name)
			next, had := doc.NextKey[container]
			item := doc.AddItem(container, name)
			*created = append(*created, createdItem{item: item, container: container, nextKey: next, hadNext: had})
			resolved[pend[pi].key] = item
			real = item
			pi++
			continue
		}
		real = real.Child(seg)
	}
	return real
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

func checkSources(f Fill, opts ApplyOptions) *Issue {
	if opts.AllowedSources == nil {
		return nil
	}
	allowed := opts.AllowedSources(f.Path)
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
