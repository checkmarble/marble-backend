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
// declaration is authorized (organization + case access). 409 if not open.
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
		cur, err := uc.generation.GetSarQuestion(ctx, tx, questionId)
		if err != nil {
			return err
		}
		if cur.Status != models.SarQuestionStatusOpen {
			return errors.Wrap(models.ConflictError, "cette question est déjà fermée")
		}
		return uc.generation.CloseSarQuestions(ctx, tx, []uuid.UUID{questionId}, models.SarQuestionStatusDismissed)
	})
}

// ChooseQuestionOption applies option optionIndex of an open question without
// LLM or job: its fills with core.OverwriteAll, keeping each fill's origin and
// sources, then core.MarkReviewed on their paths, then closes the question
// (resolved). Checks: declaration version (models.ErrSarVersionConflict),
// optionIndex in range (400), the option has fills (400), no active job
// (ErrGenerationInProgress). An option without fills is accepted only when a
// sibling option creates a new list item (provisional key): it closes the
// question without writing or bumping the version. Rejected fills (LLM-provided paths) -> 400 and
// nothing is applied, as is a fill on a setting or computed field. Returns the new declaration state.
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
		return DeclarationState{}, errors.Wrap(models.ConflictError, "cette question est déjà fermée")
	}
	if optionIndex < 0 || optionIndex >= len(q.Options) {
		return DeclarationState{}, errors.Wrap(models.BadParameterError, "option inconnue")
	}
	opt := q.Options[optionIndex]
	if len(opt.Fills) == 0 {
		if questionCreatesItem(q) {
			// « Do not add »: the option declines the addition proposed by a
			// sibling option. It closes the question and writes nothing.
			return uc.declineQuestion(ctx, q, version)
		}
		return DeclarationState{}, errors.Wrap(models.BadParameterError, "cette option ne propose aucune valeur")
	}
	if len(q.Paths) == 0 {
		// No paths means no restriction in ApplyFills: refuse rather than write anywhere.
		return DeclarationState{}, errors.Wrap(models.BadParameterError,
			"cette question ne désigne aucun champ : ses options ne peuvent rien écrire")
	}
	st, err := uc.mutateWithPost(ctx, q.DeclarationId, version,
		func(format core.Format, doc *core.Document, _, _ *string) error {
			spec := format.Spec()
			// The analyst's click is the proof: the fill takes the analyst
			// origin (no source or justification required, no task), writes
			// are restricted to the question's own paths, and the choice may
			// replace a manual value (OverwriteAll). Settings and computed
			// fields are never writable through an option.
			fills := make([]core.Fill, len(opt.Fills))
			for i, f := range opt.Fills {
				if node := spec.NodeAt(f.Path.Template()); node != nil {
					switch node.EffectiveNature() {
					case core.NatureSetting, core.NatureComputed:
						return errors.Wrapf(models.BadParameterError,
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
				f.Origin, f.TaskID = core.OriginAnalyst, nil
				fills[i] = f
			}
			if optionAlreadyPresent(doc, fills) {
				return errOptionAlreadyApplied
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
			return nil
		},
		func(tx repositories.Transaction, _ *core.Document) error {
			cur, err := uc.generation.GetSarQuestion(ctx, tx, questionId)
			if err != nil {
				return err
			}
			if cur.Status != models.SarQuestionStatusOpen {
				return errors.Wrap(models.ConflictError, "cette question est déjà fermée")
			}
			return uc.generation.CloseSarQuestions(ctx, tx, []uuid.UUID{questionId}, models.SarQuestionStatusResolved)
		})
	if errors.Is(err, errOptionAlreadyApplied) {
		// The analyst already added what the option adds: the question is
		// answered, nothing is written and the version is kept.
		return uc.declineQuestion(ctx, q, version)
	}
	return st, err
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
		for _, k := range doc.Items[g.container] {
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
			return errors.Wrap(models.ConflictError, "cette question est déjà fermée")
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
// declaration whose paths all have a non-empty value in doc, among those
// touching one of the paths just filled (origin does not matter).
func (uc *Usecase) resolveAnsweredQuestions(ctx context.Context, tx repositories.Transaction,
	declarationId uuid.UUID, doc *core.Document, filled []core.Path,
) error {
	open, err := uc.generation.ListSarQuestions(ctx, tx, declarationId, models.SarQuestionStatusOpen)
	if err != nil {
		return err
	}
	var ids []uuid.UUID
	for _, q := range open {
		if q.Kind != models.SarQuestionKindQuestion || len(q.Paths) == 0 {
			continue
		}
		touched, all := false, true
		for _, p := range q.Paths {
			if v, ok := doc.Values[core.Path(p)]; !ok || v.Value == "" {
				all = false
			}
			if slices.Contains(filled, core.Path(p)) {
				touched = true
			}
		}
		if touched && all {
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
