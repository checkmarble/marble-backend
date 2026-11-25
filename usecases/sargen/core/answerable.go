package core

import "slices"

// computedConfirmer is the optional capability of a format that lists the
// computed values the analyst may give by hand (ConfirmComputer, where the format
// interface defines it).
type computedConfirmer interface{ ConfirmComputed() []Path }

// FieldAnswerable tells whether a free answer can be written at the path: a plain
// field (not a list item, a list container or a group), not a setting nor an
// analyst text, and a computed one only when the format lets the analyst give it
// by hand. It is the single rule of the answer endpoint, of the output checks
// that must not leave a blocking item nobody can answer, and of what stays
// answerable once part of a question's paths is gone.
func FieldAnswerable(format Format, p Path) bool {
	n := format.Spec().NodeAt(p.Template())
	if n == nil || n.Kind != NodeField || n.IsRepeatable() {
		return false
	}
	switch n.EffectiveNature() {
	case NatureCopied, NatureJudgment:
		return true
	case NatureComputed:
		cc, ok := format.(computedConfirmer)
		return ok && slices.Contains(cc.ConfirmComputed(), Path(p.Template()))
	}
	return false
}
