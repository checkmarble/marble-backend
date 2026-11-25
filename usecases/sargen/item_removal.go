package sargen

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// removeItemRewritingReferences removes an item from the document after
// rewriting what refers to it, so that nothing refers to the removed item
// afterwards. It is the sequence shared by the removal through a question option
// and the removal by hand: the fills the format gives for the item
// (core.ItemRemovalRewriter, computed now from the document as it is), applied
// with the manual origin over any value and marked reviewed; the paths to clear
// deleted; the item removed with everything below it; the computed values
// recomputed. The callers check the item first (it exists, it is a list item
// that is not computed).
func (uc *Usecase) removeItemRewritingReferences(ctx context.Context, format core.Format, doc *core.Document, item core.Path) error {
	spec := format.Spec()
	var rewrite core.RemovalRewrite
	if rw, ok := format.(core.ItemRemovalRewriter); ok {
		rewrite = rw.RewriteBeforeRemoval(doc, item)
	}
	fills := rewrite.Fills
	for _, p := range rewrite.Clear {
		if node := spec.NodeAt(p.Template()); node != nil {
			switch node.EffectiveNature() {
			case core.NatureSetting, core.NatureComputed:
				return errors.Newf("rewrite of %s: a setting or a computed field is never cleared by a removal", p)
			}
		}
		delete(doc.Values, p)
	}
	for i, f := range fills {
		if node := spec.NodeAt(f.Path.Template()); node != nil {
			switch node.EffectiveNature() {
			case core.NatureSetting, core.NatureComputed:
				return errors.Newf("rewrite of %s: a setting or a computed field is never written by a removal", f.Path)
			}
		}
		f.Origin, f.TaskID = core.OriginManual, nil
		fills[i] = f
	}
	res := core.ApplyFills(spec, doc, fills, core.ApplyOptions{Overwrite: core.OverwriteAll})
	var issues []core.Issue
	for _, r := range res.Rejected {
		issues = append(issues, r.Issue)
	}
	if len(issues) > 0 {
		return issuesToError(issues)
	}
	var paths []core.Path
	for _, f := range fills {
		if f.Value != "" {
			paths = append(paths, f.Path)
		}
	}
	if issues := core.MarkReviewed(spec, doc, paths); len(issues) > 0 {
		return issuesToError(issues)
	}
	doc.RemoveItem(item)
	uc.recompute(ctx, format, doc)
	return nil
}
