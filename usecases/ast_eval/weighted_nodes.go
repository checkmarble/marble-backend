package ast_eval

import (
	"cmp"
	"slices"

	"github.com/checkmarble/marble-backend/models/ast"
)

// Weighted nodes manages a flat list of nodes and offers an interface to process
// them sorted by node cost. A lower-cost node will be executed earlier when the
// parent is commutative.
//
// The parent is passed to the constructor, so that if it is not commutative, this
// is basically a no-op.
type WeightedNodes struct {
	enabled  bool
	original []ast.Node
}

func NewWeightedNodes(env AstEvaluationEnvironment, parent ast.Node, nodes []ast.Node) WeightedNodes {
	enabled := false

	if !env.disableCostOptimizations {
		if fattrs, err := parent.Function.Attributes(); err == nil {
			enabled = fattrs.Commutative
		}
	}

	if enabled {
		for idx := range nodes {
			nodes[idx].Index = idx
		}
	}

	return WeightedNodes{
		enabled:  enabled,
		original: nodes,
	}
}

func (wn WeightedNodes) Sorted() []ast.Node {
	if !wn.enabled {
		return wn.original
	}

	return slices.SortedFunc(slices.Values(wn.original), func(lhs, rhs ast.Node) int {
		return cmp.Compare(lhs.Cost(), rhs.Cost())
	})
}

func (wn WeightedNodes) Reorder(results []ast.NodeEvaluation) []ast.NodeEvaluation {
	if !wn.enabled {
		return results
	}

	output := make([]ast.NodeEvaluation, len(wn.original))

	for idx := range wn.original {
		output[idx] = ast.NodeEvaluation{
			Index:       idx,
			Skipped:     true,
			ReturnValue: nil,
		}
	}

	for _, result := range results {
		output[result.Index] = result
		output[result.Index].Skipped = false
	}

	return output
}
