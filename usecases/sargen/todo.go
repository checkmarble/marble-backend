package sargen

import (
	"sort"

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
