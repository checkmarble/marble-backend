package engine

import (
	"context"
	"fmt"
	"slices"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// backendQuestions returns the questions the backend raises from the document,
// before the final review and again at the end of a job (idempotent): the
// questions of the format (core.DocumentQuestioner: a match to confirm, a choice
// the data do not make) and the questions about the required coded fields left
// empty. A question is never asked twice: a draft whose paths already have a
// question of the declaration, open or closed, is dropped. Nothing is written.
func (r *jobRun) backendQuestions(ctx context.Context, tx repositories.Transaction, doc *core.Document) ([]models.SarQuestion, error) {
	all, err := r.o.generation.ListSarQuestions(ctx, tx, r.declId, "")
	if err != nil {
		return nil, errors.Wrap(err, "cannot list questions")
	}
	asked := map[string]bool{}
	// skip: every path that already has a question (any status, a replaced one
	// included) or an open finding, for the questions about single fields.
	skip := map[string]bool{}
	for _, q := range all {
		switch {
		case q.Kind == models.SarQuestionKindQuestion:
			asked[pathsKey(q.Paths)] = true
		case q.Kind == models.SarQuestionKindReviewFinding && q.Status == models.SarQuestionStatusOpen:
		default:
			continue
		}
		for _, p := range q.Paths {
			skip[p] = true
		}
	}
	var out []models.SarQuestion
	if dq, ok := r.format.(core.DocumentQuestioner); ok {
		for _, d := range dq.DocumentQuestions(doc) {
			paths := make([]string, len(d.Paths))
			for i, p := range d.Paths {
				paths[i] = string(p)
			}
			if asked[pathsKey(paths)] {
				continue
			}
			q, ok := r.questionFromDraft(doc, d, paths)
			if !ok {
				continue
			}
			asked[pathsKey(paths)] = true
			out = append(out, q)
			for _, p := range paths {
				skip[p] = true
			}
		}
	}
	env := missingCodeEnv{format: r.format, selection: r.selection, dataModel: r.dataModel}
	out = append(out, missingCodeQuestions(env, doc, r.declId, skip)...)
	return out, nil
}

func pathsKey(paths []string) string {
	sorted := slices.Clone(paths)
	slices.Sort(sorted)
	return fmt.Sprint(sorted)
}

// questionFromDraft turns a question of the format into a stored one. Each
// option that writes is checked like a fill: its paths within the question's,
// never a setting or a computed field, and applied on a copy of the document. An
// option that does not pass is dropped; the options that write nothing (leave as
// it is) stay only when a writing option stays. A draft whose writing options all
// failed is dropped.
func (r *jobRun) questionFromDraft(doc *core.Document, d core.QuestionDraft, paths []string) (models.SarQuestion, bool) {
	var opts []core.QuestionOption
	wrote, hadWriting := false, false
	for _, op := range d.Options {
		op.RemoveItem = ""
		if len(op.Fills) == 0 && len(op.Clear) == 0 {
			opts = append(opts, op)
			continue
		}
		hadWriting = true
		if r.optionWritesCleanly(doc, op, d.Paths) {
			opts = append(opts, op)
			wrote = true
		}
	}
	if hadWriting && !wrote {
		return models.SarQuestion{}, false
	}
	if !wrote {
		opts = nil // only « leave as it is » options: nothing to choose
	}
	if d.Blocking && len(opts) == 0 && !slices.ContainsFunc(d.Paths, func(p core.Path) bool { return core.FieldAnswerable(r.format, p) }) {
		// A blocking question nobody can answer would close the export for good.
		return models.SarQuestion{}, false
	}
	q := models.SarQuestion{
		Id: uuid.New(), DeclarationId: r.declId, Kind: models.SarQuestionKindQuestion,
		Paths: paths, Text: d.Text, Status: models.SarQuestionStatusOpen,
	}
	if d.Blocking {
		q.Severity = models.SarQuestionSeverityBlocking
	}
	if len(opts) > 0 {
		raw, err := marshalOptions(opts)
		if err != nil {
			return models.SarQuestion{}, false
		}
		q.Options = raw
	}
	return q, true
}

// optionWritesCleanly dry-runs an option on a copy of the document, the way
// choosing it will write it: clears deleted, fills with the manual origin over
// any value, within the paths of the question, never on a setting or a computed
// field.
func (r *jobRun) optionWritesCleanly(doc *core.Document, op core.QuestionOption, paths []core.Path) bool {
	spec := r.format.Spec()
	writable := func(p core.Path) bool {
		if !core.PathAllowed(p, paths) {
			return false
		}
		if node := spec.NodeAt(p.Template()); node != nil {
			switch node.EffectiveNature() {
			case core.NatureSetting, core.NatureComputed:
				return false
			}
		}
		return true
	}
	clone := doc.Clone()
	for _, p := range op.Clear {
		if !writable(p) {
			return false
		}
		delete(clone.Values, p)
	}
	fills := make([]core.Fill, len(op.Fills))
	for i, f := range op.Fills {
		if !writable(f.Path) {
			return false
		}
		f.Origin, f.TaskID = core.OriginManual, nil
		fills[i] = f
	}
	res := core.ApplyFills(spec, clone, fills, core.ApplyOptions{
		AllowedPrefixes: containerPrefixes(paths), Overwrite: core.OverwriteAll,
	})
	return len(res.Rejected) == 0
}

// containerPrefixes cuts each provisional item segment of the paths to its
// container (`a.b[+x].c` -> `a.b`).
func containerPrefixes(paths []core.Path) []core.Path {
	out := make([]core.Path, len(paths))
	for i, p := range paths {
		out[i] = p
		for j := 0; j+1 < len(p); j++ {
			if p[j] == '[' && p[j+1] == '+' {
				out[i] = p[:j]
				break
			}
		}
	}
	return out
}

// askBackendQuestions creates the backend questions before the final review (a
// generation job also recomputes first, so that the questions and the review see
// the computed values up to date). It writes the computed values only for a
// generation job: a review job never writes the document.
func (r *jobRun) askBackendQuestions(ctx context.Context) error {
	o := r.o
	r.applyMu.Lock()
	defer r.applyMu.Unlock()
	return o.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		decl, err := o.declarations.GetSarDeclarationForUpdate(ctx, tx, r.declId)
		if err != nil {
			return errors.Wrap(err, "cannot lock declaration")
		}
		if running, err := r.stillRunning(ctx, tx); err != nil {
			return err
		} else if !running {
			return nil
		}
		doc, err := decodeDocument(decl.Document)
		if err != nil {
			return err
		}
		changed := false
		if r.job.Kind != models.SarJobKindReview {
			_, changed = r.computeInto(doc)
		}
		qs, err := r.backendQuestions(ctx, tx, doc)
		if err != nil {
			return err
		}
		if len(qs) > 0 {
			if err := o.generation.CreateSarQuestions(ctx, tx, qs); err != nil {
				return errors.Wrap(err, "cannot create questions")
			}
		}
		if changed {
			return r.saveDocument(ctx, tx, decl, doc)
		}
		return nil
	})
}
