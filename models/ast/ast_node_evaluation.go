package ast

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
)

type NodeEvaluation struct {
	// Index of the initial node winhin its level of the AST tree, used to
	// reorder the results as they were. This should become obsolete when each
	// node has a unique ID.
	Index          int
	EvaluationPlan NodeEvaluationPlan

	Function    Function
	ReturnValue any
	Errors      []error

	Children      []NodeEvaluation
	NamedChildren map[string]NodeEvaluation
}

type NodeEvaluationPlan struct {
	// Skipped indicates whether this node was evaluated at all or not. A `true` values means the
	// engine determined the result of this node would not impact the overall decision's outcome.
	Skipped bool
	// Cached indicates whether this particular evaluation was pulled from the cached
	// value of a previously=executed node.
	Cached bool
	Took   time.Duration
}

func (root NodeEvaluation) FlattenErrors() []error {
	errs := make([]error, 0)

	errs = append(errs, root.Errors...)

	for _, child := range root.Children {
		errs = append(errs, child.FlattenErrors()...)
	}

	for _, child := range root.NamedChildren {
		errs = append(errs, child.FlattenErrors()...)
	}

	return errs
}

func (root NodeEvaluation) GetBoolReturnValue() (bool, error) {
	if root.ReturnValue == nil {
		return false, ErrNullFieldRead
	}

	if returnValue, ok := root.ReturnValue.(bool); ok {
		return returnValue, nil
	}

	return false, errors.New(
		fmt.Sprintf("root ast expression does not return a boolean, '%T' instead", root.ReturnValue))
}

func (root NodeEvaluation) GetStringReturnValue() (string, error) {
	if root.ReturnValue == nil {
		return "", ErrNullFieldRead
	}

	if returnValue, ok := root.ReturnValue.(string); ok {
		return returnValue, nil
	}

	return "", errors.New(fmt.Sprintf("ast expression expected to return a string, got '%T' instead", root.ReturnValue))
}

func (root *NodeEvaluation) SetCached() {
	root.EvaluationPlan.Cached = true

	for idx := range root.Children {
		root.Children[idx].SetCached()
	}
	for key := range root.NamedChildren {
		child := root.NamedChildren[key]
		child.SetCached()

		root.NamedChildren[key] = child
	}
}

type EvaluationStats struct {
	Function     Function
	Took         time.Duration
	Nodes        int
	SkippedCount int
	CachedCount  int
	Skipped      bool
	Cached       bool
	Children     []EvaluationStats
}

func BuildEvaluationStats(root NodeEvaluation) EvaluationStats {
	stats := EvaluationStats{
		Function: root.Function,
		Took:     root.EvaluationPlan.Took,
		Nodes:    len(root.Children) + len(root.NamedChildren),
		Children: make([]EvaluationStats, len(root.Children),
			len(root.Children)+len(root.NamedChildren)),
	}

	if root.EvaluationPlan.Skipped {
		stats.Skipped = true
		stats.SkippedCount = 1
	}
	if root.EvaluationPlan.Cached {
		stats.Skipped = true
		stats.CachedCount = 1
	}

	for idx, child := range root.Children {
		stats.Children[idx] = BuildEvaluationStats(child)

		stats.Nodes += stats.Children[idx].Nodes
		stats.SkippedCount += stats.Children[idx].SkippedCount
		stats.CachedCount += stats.Children[idx].CachedCount
	}
	for _, child := range root.NamedChildren {
		namedChildrenStats := BuildEvaluationStats(child)

		stats.Nodes += namedChildrenStats.Nodes
		stats.SkippedCount += namedChildrenStats.SkippedCount
		stats.CachedCount += namedChildrenStats.CachedCount

		stats.Children = append(stats.Children, namedChildrenStats)
	}

	return stats
}

type FunctionStats struct {
	Count   int           `json:"count"`
	Cached  int           `json:"cached"`
	Skipped int           `json:"skipped"`
	Took    time.Duration `json:"took"`
}

func (stats EvaluationStats) FunctionStats() map[string]FunctionStats {
	acc := make(map[string]FunctionStats)

	buildFunctionStats(acc, stats)

	return acc
}

func buildFunctionStats(acc map[string]FunctionStats, stats EvaluationStats) {
	f := stats.Function.DebugString()

	if _, ok := acc[f]; !ok {
		acc[f] = FunctionStats{}
	}

	stat := acc[f]
	stat.Count += 1
	stat.Took += stats.Took

	if stats.Skipped {
		stat.Skipped += 1
	}
	if stats.Cached {
		stat.Cached += 1
	}

	acc[f] = stat

	for _, child := range stats.Children {
		buildFunctionStats(acc, child)
	}
}
