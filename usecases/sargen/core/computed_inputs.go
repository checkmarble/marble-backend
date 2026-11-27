package core

import "slices"

// ComputedInputsMapper is an optional capability of a Format: for a computed
// path of the document, the paths of the input fields it is computed from (the
// fields an analyst can fill so that the computation produces a value). It
// answers nothing (nil) for a path that is not computed or has no such inputs.
// It never writes the document.
type ComputedInputsMapper interface {
	ComputedInputFields(doc *Document, p Path) []Path
}

// ResolveComputedPaths replaces each path that is a computed field the format
// knows the inputs of by those input paths, keeps the other paths, and removes
// duplicates. The order is stable: a path is replaced in place, and a path
// appears at its first occurrence. A format without the capability returns the
// paths unchanged (duplicates removed).
func ResolveComputedPaths(format Format, doc *Document, paths []Path) []Path {
	mapper, _ := format.(ComputedInputsMapper)
	out := make([]Path, 0, len(paths))
	add := func(p Path) {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	for _, p := range paths {
		if mapper != nil {
			if inputs := mapper.ComputedInputFields(doc, p); len(inputs) > 0 {
				for _, in := range inputs {
					add(in)
				}
				continue
			}
		}
		add(p)
	}
	return out
}
