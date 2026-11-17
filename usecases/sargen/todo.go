package sargen

import (
	"slices"
	"sort"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// TodoCodeComputedToConfirm is the stable Code of the « valeur calculée à
// confirmer » item.
const TodoCodeComputedToConfirm = "computed_to_confirm"

// buildTodoBase: issues and low-confidence values, sorted blocking, review, warning.
func buildTodoBase(issues []core.Issue, doc *core.Document) []TodoItem {
	var todo []TodoItem
	for _, i := range issues {
		sev := TodoWarning
		if i.Severity == core.SeverityBlocking {
			sev = TodoBlocking
		}
		paths, msg := []core.Path{i.Path}, i.Message
		if inputs := computedSources(doc, i.Path); inputs != nil && i.Severity == core.SeverityBlocking {
			// A computed value cannot be corrected itself: its source fields can.
			paths = append(paths, inputs...)
			msg += " — corrigez les transactions sources"
		}
		todo = append(todo, TodoItem{Kind: TodoIssue, Severity: sev, Paths: paths, Message: msg})
	}
	if doc != nil {
		var low []core.Path
		for p, v := range doc.Values {
			if v.Confidence == core.ConfidenceLow && !v.Reviewed {
				low = append(low, p)
			}
		}
		sort.Slice(low, func(a, b int) bool { return low[a] < low[b] })
		for _, p := range low {
			todo = append(todo, TodoItem{
				Kind: TodoLowConfidence, Severity: TodoReview, Paths: []core.Path{p},
				Message: "valeur à faible confiance, à relire",
			})
		}
	}
	rank := map[TodoSeverity]int{TodoBlocking: 0, TodoReview: 1, TodoWarning: 2}
	sort.SliceStable(todo, func(a, b int) bool { return rank[todo[a].Severity] < rank[todo[b].Severity] })
	return todo
}

// buildTodo returns one TodoIssue entry per issue, one TodoLowConfidence entry per unreviewed
// low-confidence value, plus, from the open questions and the
// spec: one TodoQuestion per open question (blocking when one of its paths is
// required, i.e. Min >= 1 in the spec, else warning), one TodoReviewFinding
// (review) per open review finding, and one review entry « valeur calculée à
// confirmer » per NatureComputed field outside a repeatable item that has a
// value which is not Reviewed.
func buildTodo(spec *core.Spec, confirm []core.Path, issues []core.Issue, doc *core.Document, open []models.SarQuestion) []TodoItem {
	todo := buildTodoBase(issues, doc)
	for _, q := range open {
		id := q.Id
		paths := make([]core.Path, len(q.Paths))
		for i, p := range q.Paths {
			paths[i] = core.Path(p)
		}
		switch q.Kind {
		case models.SarQuestionKindReviewFinding:
			todo = append(todo, TodoItem{Kind: TodoReviewFinding, Severity: TodoReview, Paths: paths, Message: q.Text, RefId: &id})
		default:
			sev := TodoWarning
			for _, p := range paths {
				if spec == nil {
					break
				}
				if n := spec.NodeAt(p.Template()); n != nil && n.Min >= 1 {
					sev = TodoBlocking
					break
				}
			}
			todo = append(todo, TodoItem{Kind: TodoQuestion, Severity: sev, Paths: paths, Message: q.Text, RefId: &id})
		}
	}
	if doc != nil && spec != nil && spec.Root != nil {
		var computed []core.Path
		for p, v := range doc.Values {
			if v.Value == "" || v.Reviewed || strings.Contains(p.Template(), "[]") {
				continue
			}
			if n := spec.NodeAt(p.Template()); n != nil && n.EffectiveNature() == core.NatureComputed &&
				slices.Contains(confirm, core.Path(p.Template())) {
				computed = append(computed, p)
			}
		}
		sort.Slice(computed, func(a, b int) bool { return computed[a] < computed[b] })
		for _, p := range computed {
			todo = append(todo, TodoItem{
				Kind: TodoIssue, Severity: TodoReview, Code: TodoCodeComputedToConfirm, Paths: []core.Path{p},
				Message: "valeur calculée à confirmer",
			})
		}
	}
	rank := map[TodoSeverity]int{TodoBlocking: 0, TodoReview: 1, TodoWarning: 2}
	sort.SliceStable(todo, func(a, b int) bool { return rank[todo[a].Severity] < rank[todo[b].Severity] })
	return todo
}

// confirmComputedOf returns the computed values the format wants the analyst to
// confirm (core.ConfirmComputer); none by default.
func confirmComputedOf(f core.Format) []core.Path {
	if c, ok := f.(core.ConfirmComputer); ok {
		return c.ConfirmComputed()
	}
	return nil
}

// computedSources returns the source paths of a computed value: those of the
// value at p, or, when p is a missing field of a computed line, those of the
// line's other computed values (the same transactions fed them).
func computedSources(doc *core.Document, p core.Path) []core.Path {
	if v, ok := doc.ValueAt(p); ok && v.Computation != nil {
		return v.Computation.Inputs
	}
	if doc == nil || p.Parent() == "" || !strings.HasSuffix(string(p.Parent()), "]") {
		return nil
	}
	line := p.Parent()
	var names []string
	for q, v := range doc.Values {
		if v.Computation != nil && q.Parent() == line {
			names = append(names, string(q))
		}
	}
	sort.Strings(names)
	for _, q := range names {
		if in := doc.Values[core.Path(q)].Computation.Inputs; len(in) > 0 {
			return in
		}
	}
	return nil
}
