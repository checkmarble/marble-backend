package sargen

import (
	"sort"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// buildTodo returns one TodoIssue entry per issue plus one TodoLowConfidence
// entry (severity review) per ConfidenceLow value that is not Reviewed.
// Sorted: blocking, review, warning.
func buildTodo(issues []core.Issue, doc *core.Document) []TodoItem {
	var todo []TodoItem
	for _, i := range issues {
		sev := TodoWarning
		if i.Severity == core.SeverityBlocking {
			sev = TodoBlocking
		}
		todo = append(todo, TodoItem{Kind: TodoIssue, Severity: sev, Paths: []core.Path{i.Path}, Message: i.Message})
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

// buildTodoWithQuestions is buildTodo plus, from the open questions and the
// spec: one TodoQuestion per open question (blocking when one of its paths is
// required, i.e. Min >= 1 in the spec, else warning), one TodoReviewFinding
// (review) per open review finding, and one review entry « valeur calculée à
// confirmer » per NatureComputed field outside a repeatable item that has a
// value which is not Reviewed.
func buildTodoWithQuestions(spec *core.Spec, issues []core.Issue, doc *core.Document, open []models.SarQuestion) []TodoItem {
	todo := buildTodo(issues, doc)
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
				if n := spec.NodeAt(p.Template()); n != nil && n.Min >= 1 {
					sev = TodoBlocking
					break
				}
			}
			todo = append(todo, TodoItem{Kind: TodoQuestion, Severity: sev, Paths: paths, Message: q.Text, RefId: &id})
		}
	}
	if doc != nil && spec != nil {
		var computed []core.Path
		for p, v := range doc.Values {
			if v.Value == "" || v.Reviewed || strings.Contains(p.Template(), "[]") {
				continue
			}
			if n := spec.NodeAt(p.Template()); n != nil && n.EffectiveNature() == core.NatureComputed {
				computed = append(computed, p)
			}
		}
		sort.Slice(computed, func(a, b int) bool { return computed[a] < computed[b] })
		for _, p := range computed {
			todo = append(todo, TodoItem{
				Kind: TodoIssue, Severity: TodoReview, Paths: []core.Path{p},
				Message: "valeur calculée à confirmer",
			})
		}
	}
	rank := map[TodoSeverity]int{TodoBlocking: 0, TodoReview: 1, TodoWarning: 2}
	sort.SliceStable(todo, func(a, b int) bool { return rank[todo[a].Severity] < rank[todo[b].Severity] })
	return todo
}
