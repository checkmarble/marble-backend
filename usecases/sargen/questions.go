package sargen

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Question is the usecase view of a models.SarQuestion: Options decoded from
// the stored JSON ([]core.QuestionOption; empty for a review finding).
type Question struct {
	Id            uuid.UUID
	DeclarationId uuid.UUID
	TaskId        *uuid.UUID
	// Kind is models.SarQuestionKindQuestion or models.SarQuestionKindReviewFinding.
	Kind string
	// Severity is set for review findings: "important" | "mineur".
	Severity string
	Paths    []core.Path
	Text     string
	Options  []core.QuestionOption
	// Status is models.SarQuestionStatusOpen, ...Resolved or ...Dismissed.
	Status    string
	CreatedAt time.Time
	ClosedAt  *time.Time
}

// ListQuestions returns every question and review finding of a declaration
// (open, resolved and dismissed), oldest first. Access through the declaration.
func (uc *Usecase) ListQuestions(ctx context.Context, declarationId uuid.UUID) ([]Question, error) {
	exec := uc.executorFactory.NewExecutor()
	if _, err := uc.loadDeclaration(ctx, exec, declarationId); err != nil {
		return nil, err
	}
	rows, err := uc.generation.ListSarQuestions(ctx, exec, declarationId, "")
	if err != nil {
		return nil, err
	}
	out := make([]Question, 0, len(rows))
	for _, r := range rows {
		q, err := questionFromModel(r)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, nil
}

// DismissQuestion closes a question or review finding as dismissed (« ignoré »
// / « traité » for a finding). The question is loaded by id, then its
// declaration is authorized (organization + case access). 409 if not open,
// ErrGenerationInProgress while a job is active, like every other write.
func (uc *Usecase) DismissQuestion(ctx context.Context, questionId uuid.UUID) error {
	exec := uc.executorFactory.NewExecutor()
	q, err := uc.generation.GetSarQuestion(ctx, exec, questionId)
	if err != nil {
		return err
	}
	if _, err := uc.loadDeclaration(ctx, exec, q.DeclarationId); err != nil {
		return err
	}
	return uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if _, err := uc.repository.GetSarDeclarationForUpdate(ctx, tx, q.DeclarationId); err != nil {
			return err
		}
		if err := uc.requireNoActiveJob(ctx, tx, q.DeclarationId); err != nil {
			return err
		}
		cur, err := uc.generation.GetSarQuestion(ctx, tx, questionId)
		if err != nil {
			return err
		}
		if cur.Status != models.SarQuestionStatusOpen {
			return models.NewSarError(models.ConflictError, models.SarCodeQuestionClosed, "cette question est déjà fermée")
		}
		if cur.Severity == models.SarQuestionSeverityBlocking {
			// A blocking question keeps the export closed until it is answered: it is
			// never ignored, one of its options must be chosen.
			return models.NewSarError(models.ConflictError, models.SarCodeQuestionBlocking,
				"cette question doit être tranchée par une de ses options")
		}
		return uc.generation.CloseSarQuestions(ctx, tx, []uuid.UUID{questionId}, models.SarQuestionStatusDismissed)
	})
}

// ChooseQuestionOption applies option optionIndex of an open question without
// LLM or job: its fills with core.OverwriteAll and core.OriginManual (the
// click is the proof: no source or justification required, no task; the
// sources of the option are kept), then core.MarkReviewed on their
// paths, then closes the question (resolved) and the other open questions
// whose paths are now all filled (resolveAnsweredQuestions, like a manual
// edit; a question that adds a list item is never closed that way). Checks: declaration version (models.ErrSarVersionConflict),
// optionIndex in range (400), the option has fills (400), no active job
// (ErrGenerationInProgress). An option without fills is accepted only when a
// sibling option creates a new list item (provisional key): it closes the
// question without writing or bumping the version. A fill outside the
// question's paths -> 400 and nothing is applied, as is a fill on a setting or
// computed field. Returns the new declaration state.
func (uc *Usecase) ChooseQuestionOption(ctx context.Context, questionId uuid.UUID, version int, optionIndex int) (DeclarationState, error) {
	exec := uc.executorFactory.NewExecutor()
	row, err := uc.generation.GetSarQuestion(ctx, exec, questionId)
	if err != nil {
		return DeclarationState{}, err
	}
	if _, err := uc.loadDeclaration(ctx, exec, row.DeclarationId); err != nil {
		return DeclarationState{}, err
	}
	q, err := questionFromModel(row)
	if err != nil {
		return DeclarationState{}, err
	}
	if q.Status != models.SarQuestionStatusOpen {
		return DeclarationState{}, models.NewSarError(models.ConflictError, models.SarCodeQuestionClosed, "cette question est déjà fermée")
	}
	if optionIndex < 0 || optionIndex >= len(q.Options) {
		return DeclarationState{}, models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest, "option inconnue")
	}
	opt := q.Options[optionIndex]
	if opt.RemoveItem != "" {
		return uc.chooseRemoval(ctx, q, opt, version)
	}
	if len(opt.Fills) == 0 && len(opt.Clear) == 0 {
		if questionCreatesItem(q) || questionRemovesItem(q) || questionWrites(q) {
			// « Do not add » / « keep » / « leave as it is »: the option declines
			// what a sibling option proposes (a new list item, the removal of an
			// item, values or clears). It closes the question and writes nothing.
			return uc.declineQuestion(ctx, q, version)
		}
		return DeclarationState{}, models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest, "cette option ne propose aucune valeur")
	}
	if len(q.Paths) == 0 {
		// No paths means no restriction in ApplyFills: refuse rather than write anywhere.
		return DeclarationState{}, models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest,
			"cette question ne désigne aucun champ : ses options ne peuvent rien écrire")
	}
	var written []core.Path
	st, err := uc.mutateWithPost(ctx, q.DeclarationId, version,
		func(format core.Format, doc *core.Document, _, _ *string) error {
			spec := format.Spec()
			// The analyst's click is the proof: the fill takes the analyst
			// origin (no source or justification required, no task), writes
			// are restricted to the question's own paths, and the choice may
			// replace a manual value (OverwriteAll). Settings and computed
			// fields are never writable through an option.
			for _, p := range opt.Clear {
				if node := spec.NodeAt(p.Template()); node != nil {
					switch node.EffectiveNature() {
					case core.NatureSetting, core.NatureComputed:
						return models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest,
							"le champ %s est un réglage ou un champ calculé : il ne peut pas être vidé par une option", p)
					}
				}
				if !core.PathAllowed(p, q.Paths) {
					return issuesToError([]core.Issue{{
						Path: p, Code: core.CodeOutOfScope, Severity: core.SeverityBlocking,
						Message: "chemin hors des champs de la question",
					}})
				}
			}
			fills := make([]core.Fill, len(opt.Fills))
			for i, f := range opt.Fills {
				if node := spec.NodeAt(f.Path.Template()); node != nil {
					switch node.EffectiveNature() {
					case core.NatureSetting, core.NatureComputed:
						return models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest,
							"le champ %s est un réglage ou un champ calculé : il ne peut pas être renseigné par une option", f.Path)
					}
				}
				// Raw paths against the question's paths, both with their
				// provisional keys: every fill sharing `[+a]` stays within the
				// question once the first one has created the item.
				if !core.PathAllowed(f.Path, q.Paths) {
					return issuesToError([]core.Issue{{
						Path: f.Path, Code: core.CodeOutOfScope, Severity: core.SeverityBlocking,
						Message: "chemin hors des champs de la question",
					}})
				}
				f.Origin, f.TaskID = core.OriginManual, nil
				fills[i] = f
			}
			if len(opt.Clear) == 0 && optionAlreadyPresent(doc, fills) {
				return errOptionAlreadyApplied
			}
			// The cleared values are deleted first (no value is left stored there),
			// then the fills are written.
			for _, p := range opt.Clear {
				delete(doc.Values, p)
			}
			res := core.ApplyFills(spec, doc, fills, core.ApplyOptions{
				// A provisional item segment is cut to its container so that the
				// item created by the first fill is allowed for the next ones.
				AllowedPrefixes: containerPrefixes(q.Paths), Overwrite: core.OverwriteAll,
			})
			var issues []core.Issue
			for _, r := range res.Rejected {
				issues = append(issues, r.Issue)
			}
			if len(issues) > 0 {
				return issuesToError(issues)
			}
			var paths []core.Path
			for _, f := range fills {
				if f.Value != "" { // an empty fill clears the value: nothing to mark
					// A provisional item key was resolved to the created item.
					paths = append(paths, res.Rewrite(f.Path))
				}
			}
			if issues := core.MarkReviewed(spec, doc, paths); len(issues) > 0 {
				return issuesToError(issues)
			}
			written = paths
			// Computed values follow the chosen option, in the same write.
			uc.recompute(ctx, format, doc)
			return nil
		},
		func(tx repositories.Transaction, doc *core.Document) error {
			cur, err := uc.generation.GetSarQuestion(ctx, tx, questionId)
			if err != nil {
				return err
			}
			if cur.Status != models.SarQuestionStatusOpen {
				return models.NewSarError(models.ConflictError, models.SarCodeQuestionClosed, "cette question est déjà fermée")
			}
			if err := uc.generation.CloseSarQuestions(ctx, tx, []uuid.UUID{questionId}, models.SarQuestionStatusResolved); err != nil {
				return err
			}
			// The other open questions on the paths just written are answered too.
			return uc.resolveAnsweredQuestions(ctx, tx, q.DeclarationId, doc, written)
		})
	if errors.Is(err, errOptionAlreadyApplied) {
		// The analyst already added what the option adds: the question is
		// answered, nothing is written and the version is kept.
		return uc.declineQuestion(ctx, q, version)
	}
	return st, err
}

// AnswerQuestion answers an open question with a free value, for the questions
// that have no option (or none that fits). The value is written exactly like a
// manual edit (origin manual, same validation: a refused value is an
// sar_invalid_field with its issues), then marked reviewed: answering is the
// analyst's review of the field. path must be one of the question's
// paths; it may be empty when the question has a single path. The target must
// be a plain field the analyst may edit: not a list item or a group, not a
// setting, not a computed field, not an analyst text. The question is closed
// (resolved), and so are the other open questions whose paths are now all
// filled. Checks: question open, declaration version
// (models.ErrSarVersionConflict), no active job. Blocking questions and review
// findings can be answered. Returns the new declaration state.
func (uc *Usecase) AnswerQuestion(ctx context.Context, questionId uuid.UUID, version int, value string, path core.Path) (DeclarationState, error) {
	exec := uc.executorFactory.NewExecutor()
	row, err := uc.generation.GetSarQuestion(ctx, exec, questionId)
	if err != nil {
		return DeclarationState{}, err
	}
	if _, err := uc.loadDeclaration(ctx, exec, row.DeclarationId); err != nil {
		return DeclarationState{}, err
	}
	q, err := questionFromModel(row)
	if err != nil {
		return DeclarationState{}, err
	}
	if q.Status != models.SarQuestionStatusOpen {
		return DeclarationState{}, models.NewSarError(models.ConflictError, models.SarCodeQuestionClosed, "cette question est déjà fermée")
	}
	target, err := answerTarget(q, path)
	if err != nil {
		return DeclarationState{}, err
	}
	return uc.mutateWithPost(ctx, q.DeclarationId, version,
		func(format core.Format, doc *core.Document, _, _ *string) error {
			n := format.Spec().NodeAt(target.Template())
			if n == nil || n.Kind != core.NodeField || n.IsRepeatable() {
				return models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest,
					"cette question se tranche par une de ses options")
			}
			if !core.FieldAnswerable(format, target) {
				return models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest,
					"ce champ est un réglage, un champ calculé ou un texte d'analyse : il ne se renseigne pas en répondant à une question")
			}
			if _, err := uc.applyManualFills(ctx, format, doc, []core.Fill{{Path: target, Value: value}}); err != nil {
				return err
			}
			// Answering the question is the analyst's review of the field,
			// like choosing an option; a plain manual edit is not.
			if value == "" {
				return nil
			}
			if issues := core.MarkReviewed(format.Spec(), doc, []core.Path{target}); len(issues) > 0 {
				return issuesToError(issues)
			}
			return nil
		},
		func(tx repositories.Transaction, doc *core.Document) error {
			cur, err := uc.generation.GetSarQuestion(ctx, tx, questionId)
			if err != nil {
				return err
			}
			if cur.Status != models.SarQuestionStatusOpen {
				return models.NewSarError(models.ConflictError, models.SarCodeQuestionClosed, "cette question est déjà fermée")
			}
			if err := uc.generation.CloseSarQuestions(ctx, tx, []uuid.UUID{questionId}, models.SarQuestionStatusResolved); err != nil {
				return err
			}
			return uc.resolveAnsweredQuestions(ctx, tx, q.DeclarationId, doc, []core.Path{target})
		})
}

// answerTarget picks the field a free answer writes: path when given (it must
// be one of the question's paths), the only path of the question otherwise.
func answerTarget(q Question, path core.Path) (core.Path, error) {
	if path == "" {
		switch len(q.Paths) {
		case 1:
			return q.Paths[0], nil
		case 0:
			return "", models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest,
				"cette question ne désigne aucun champ : elle ne peut pas recevoir de réponse libre")
		default:
			return "", models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest,
				"cette question porte sur plusieurs champs : indiquez celui auquel répondre")
		}
	}
	if !slices.Contains(q.Paths, path) {
		return "", models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest,
			"ce champ ne fait pas partie des champs de la question")
	}
	return path, nil
}

// chooseRemoval applies an option that removes an item of the question. In one
// write: the fills the format gives for the item about to be removed
// (core.ItemRemovalRewriter, computed now from the current document), applied
// with the manual origin over any value, then the item removed with everything
// below it, then the computed values recomputed. The question is closed as
// resolved, and so are the other open questions about the removed item. An
// item that is already gone only closes the question. A stale version is a
// conflict like any write. The rewrite and the removal are the sequence shared
// with RemoveItem (removeItemRewritingReferences).
func (uc *Usecase) chooseRemoval(ctx context.Context, q Question, opt core.QuestionOption, version int) (DeclarationState, error) {
	item := opt.RemoveItem
	if !core.PathAllowed(item, q.Paths) {
		return DeclarationState{}, models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest, "l'élément à retirer ne fait pas partie des champs de la question")
	}
	var removalFormat core.Format
	st, err := uc.mutateWithPost(ctx, q.DeclarationId, version,
		func(format core.Format, doc *core.Document, _, _ *string) error {
			removalFormat = format
			spec := format.Spec()
			// Same checks as the removal of an item by hand: a list item, not of a
			// container the backend computes.
			n := spec.NodeAt(item.Template())
			if !strings.HasSuffix(string(item), "]") || n == nil || !n.IsRepeatable() {
				return models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest, "%s n'est pas un élément de liste", item)
			}
			if onlyComputed(n) {
				return models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest,
					"%s est calculée : les éléments ne se retirent pas à la main", item)
			}
			if !doc.HasItem(item) {
				return errOptionAlreadyApplied
			}
			return uc.removeItemRewritingReferences(ctx, format, doc, item)
		},
		func(tx repositories.Transaction, doc *core.Document) error {
			cur, err := uc.generation.GetSarQuestion(ctx, tx, q.Id)
			if err != nil {
				return err
			}
			if cur.Status != models.SarQuestionStatusOpen {
				return models.NewSarError(models.ConflictError, models.SarCodeQuestionClosed, "cette question est déjà fermée")
			}
			if err := uc.generation.CloseSarQuestions(ctx, tx, []uuid.UUID{q.Id}, models.SarQuestionStatusResolved); err != nil {
				return err
			}
			return uc.dismissQuestionsOfItem(ctx, tx, removalFormat, q.DeclarationId, item)
		})
	if errors.Is(err, errOptionAlreadyApplied) {
		return uc.declineQuestion(ctx, q, version)
	}
	return st, err
}

// dismissQuestionsOfItem settles the open questions and findings (every kind,
// blocking ones included) that have a path inside the removed item. One whose
// paths are all inside it has nothing left to answer: it is closed (dismissed).
// One with only some of its paths inside keeps the others: the old row is closed
// (replaced) and a new open one carries the paths that are left and the options
// that still write something there; if what is left cannot be answered (no
// option, no path a free answer can write), it is closed (dismissed) as well.
func (uc *Usecase) dismissQuestionsOfItem(ctx context.Context, tx repositories.Transaction,
	format core.Format, declarationId uuid.UUID, item core.Path,
) error {
	open, err := uc.generation.ListSarQuestions(ctx, tx, declarationId, models.SarQuestionStatusOpen)
	if err != nil {
		return err
	}
	var dismissed, replaced []uuid.UUID
	var rewritten []models.SarQuestion
	for _, oq := range open {
		var kept []string
		inside := false
		for _, p := range oq.Paths {
			if core.Path(p).HasPrefix(item) {
				inside = true
			} else {
				kept = append(kept, p)
			}
		}
		if !inside {
			continue
		}
		if len(kept) == 0 {
			dismissed = append(dismissed, oq.Id)
			continue
		}
		q, err := questionFromModel(oq)
		if err != nil {
			return err
		}
		var keptPaths []core.Path
		for _, p := range kept {
			keptPaths = append(keptPaths, core.Path(p))
		}
		opts := optionsWithoutItem(q.Options, item)
		if len(opts) == 0 && !slices.ContainsFunc(keptPaths, func(p core.Path) bool { return core.FieldAnswerable(format, p) }) {
			dismissed = append(dismissed, oq.Id)
			continue
		}
		nq := oq
		nq.Id, nq.Paths, nq.Options, nq.Status, nq.ClosedAt = uuid.New(), kept, nil, models.SarQuestionStatusOpen, nil
		if len(opts) > 0 {
			if nq.Options, err = json.Marshal(opts); err != nil {
				return errors.Wrap(err, "cannot encode question options")
			}
		}
		replaced = append(replaced, oq.Id)
		rewritten = append(rewritten, nq)
	}
	if err := uc.generation.CloseSarQuestions(ctx, tx, dismissed, models.SarQuestionStatusDismissed); err != nil {
		return err
	}
	if err := uc.generation.CloseSarQuestions(ctx, tx, replaced, models.SarQuestionStatusReplaced); err != nil {
		return err
	}
	if len(rewritten) == 0 {
		return nil
	}
	return uc.generation.CreateSarQuestions(ctx, tx, rewritten)
}

// optionsWithoutItem prunes the options of a question once an item is gone: the
// fills and clears inside the item are dropped, an option that wrote only there
// (or removed that item) disappears, and the options that write nothing (leave
// as it is) stay only when a writing option stays.
func optionsWithoutItem(opts []core.QuestionOption, item core.Path) []core.QuestionOption {
	var out []core.QuestionOption
	writes := false
	for _, o := range opts {
		wrote := len(o.Fills) > 0 || len(o.Clear) > 0 || o.RemoveItem != ""
		if o.RemoveItem != "" && o.RemoveItem.HasPrefix(item) {
			continue
		}
		var fills []core.Fill
		for _, f := range o.Fills {
			if !f.Path.HasPrefix(item) {
				fills = append(fills, f)
			}
		}
		var clear []core.Path
		for _, p := range o.Clear {
			if !p.HasPrefix(item) {
				clear = append(clear, p)
			}
		}
		if wrote && len(fills) == 0 && len(clear) == 0 && o.RemoveItem == "" {
			continue
		}
		o.Fills, o.Clear = fills, clear
		out = append(out, o)
		writes = writes || wrote
	}
	if !writes {
		return nil
	}
	return out
}

// questionRemovesItem reports whether an option of q removes an item.
func questionRemovesItem(q Question) bool {
	return slices.ContainsFunc(q.Options, func(o core.QuestionOption) bool { return o.RemoveItem != "" })
}

// errOptionAlreadyApplied tells that the chosen option would add an item the
// list already holds.
var errOptionAlreadyApplied = errors.New("option already applied")

// containerPrefixes cuts each provisional item segment of the paths to its
// container (`a.b[+x].c` -> `a.b`).
func containerPrefixes(paths []core.Path) []core.Path {
	out := make([]core.Path, len(paths))
	for i, p := range paths {
		out[i] = p
		if j := strings.Index(string(p), "[+"); j >= 0 {
			out[i] = core.Path(string(p)[:j])
		}
	}
	return out
}

// optionAlreadyPresent reports whether every fill creates, under a provisional
// item key, an item whose values equal those of an existing item of its
// container (e.g. the same code already in a list). Fills nested under a
// second provisional key are not comparable: the option is then applied.
func optionAlreadyPresent(doc *core.Document, fills []core.Fill) bool {
	type group struct {
		container core.Path
		suffixes  map[string]string // path below the item -> value
	}
	groups := map[string]*group{}
	var order []string
	for _, f := range fills {
		s := string(f.Path)
		j := strings.Index(s, "[+")
		if j < 0 || f.Value == "" {
			return false
		}
		end := strings.Index(s[j:], "]")
		if end < 0 {
			return false
		}
		literal, suffix := s[:j+end+1], s[j+end+1:]
		if strings.Contains(suffix, "[+") {
			return false
		}
		g := groups[literal]
		if g == nil {
			g = &group{container: core.Path(s[:j]), suffixes: map[string]string{}}
			groups[literal] = g
			order = append(order, literal)
		}
		g.suffixes[suffix] = f.Value
	}
	if len(order) == 0 {
		return false
	}
	for _, lit := range order {
		g := groups[lit]
		found := false
		for _, k := range doc.ItemKeys(g.container) {
			item := string(g.container) + "[" + k + "]"
			match := true
			for suf, val := range g.suffixes {
				if v, ok := doc.Values[core.Path(item+suf)]; !ok || v.Value != val {
					match = false
					break
				}
			}
			if match {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// questionWrites reports whether an option of q writes something (values or
// clears), so that an option that writes nothing is a choice to leave as it is.
func questionWrites(q Question) bool {
	return slices.ContainsFunc(q.Options, func(o core.QuestionOption) bool {
		return len(o.Fills) > 0 || len(o.Clear) > 0
	})
}

// questionCreatesItem reports whether an option of q fills a path holding a
// provisional key, i.e. choosing it creates a new list item.
func questionCreatesItem(q Question) bool {
	for _, o := range q.Options {
		for _, f := range o.Fills {
			if f.Path.HasProvisionalKey() {
				return true
			}
		}
	}
	return false
}

// declineQuestion closes an open question as resolved (answered, not
// dismissed) without writing to the document: the version is not bumped. The
// declaration version is still checked, and no job may be active, like any
// choice of an option.
func (uc *Usecase) declineQuestion(ctx context.Context, q Question, version int) (DeclarationState, error) {
	exec := uc.executorFactory.NewExecutor()
	var saved models.SarDeclaration
	err := uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		d, err := uc.repository.GetSarDeclarationForUpdate(ctx, tx, q.DeclarationId)
		if err != nil {
			return err
		}
		if err := uc.checkDeclarationAccess(ctx, tx, d); err != nil {
			return err
		}
		if err := uc.requireNoActiveJob(ctx, tx, q.DeclarationId); err != nil {
			return err
		}
		if d.Version != version {
			return models.ErrSarVersionConflict
		}
		cur, err := uc.generation.GetSarQuestion(ctx, tx, q.Id)
		if err != nil {
			return err
		}
		if cur.Status != models.SarQuestionStatusOpen {
			return models.NewSarError(models.ConflictError, models.SarCodeQuestionClosed, "cette question est déjà fermée")
		}
		saved = d
		return uc.generation.CloseSarQuestions(ctx, tx, []uuid.UUID{q.Id}, models.SarQuestionStatusResolved)
	})
	if err != nil {
		return DeclarationState{}, err
	}
	return uc.buildState(ctx, exec, saved)
}

// resolveAnsweredQuestions closes (resolved) the open questions of the
// declaration (questions and review findings of any severity) whose paths all
// have a non-empty value in doc, among those touching one of the paths just
// filled (origin does not matter). The element paths of a finding are not
// fields: they do not count. A finding without paths is never closed here.
func (uc *Usecase) resolveAnsweredQuestions(ctx context.Context, tx repositories.Transaction,
	declarationId uuid.UUID, doc *core.Document, filled []core.Path,
) error {
	open, err := uc.generation.ListSarQuestions(ctx, tx, declarationId, models.SarQuestionStatusOpen)
	if err != nil {
		return err
	}
	var ids []uuid.UUID
	for _, q := range open {
		finding := q.Kind == models.SarQuestionKindReviewFinding
		if (q.Kind != models.SarQuestionKindQuestion && !finding) || len(q.Paths) == 0 {
			continue
		}
		touched, all, fields := false, true, 0
		for _, p := range q.Paths {
			if finding && strings.HasSuffix(p, "]") {
				continue // a finding is answered by its fields, an element path cannot be filled
			}
			fields++
			if v, ok := doc.Values[core.Path(p)]; !ok || v.Value == "" {
				all = false
			}
			if slices.Contains(filled, core.Path(p)) {
				touched = true
			}
		}
		if fields > 0 && touched && all {
			ids = append(ids, q.Id)
		}
	}
	return uc.generation.CloseSarQuestions(ctx, tx, ids, models.SarQuestionStatusResolved)
}

// questionFromModel decodes the stored options JSON.
func questionFromModel(q models.SarQuestion) (Question, error) {
	out := Question{
		Id: q.Id, DeclarationId: q.DeclarationId, TaskId: q.TaskId, Kind: q.Kind,
		Severity: q.Severity, Text: q.Text, Status: q.Status, CreatedAt: q.CreatedAt, ClosedAt: q.ClosedAt,
	}
	for _, p := range q.Paths {
		out.Paths = append(out.Paths, core.Path(p))
	}
	if len(q.Options) > 0 {
		if err := json.Unmarshal(q.Options, &out.Options); err != nil {
			return Question{}, errors.Wrap(err, "cannot decode question options")
		}
	}
	return out, nil
}
