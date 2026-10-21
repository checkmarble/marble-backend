package ast_eval

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"golang.org/x/sync/singleflight"
)

type EvaluationCache struct {
	Cache    sync.Map
	Executor *singleflight.Group
}

func NewEvaluationCache() *EvaluationCache {
	return &EvaluationCache{
		Cache:    sync.Map{},
		Executor: new(singleflight.Group),
	}
}

func EvaluateAst(ctx context.Context, cache *EvaluationCache,
	environment AstEvaluationEnvironment, node ast.Node,
) (ast.NodeEvaluation, bool) {
	start := time.Now()

	// Early exit for constant, because it should have no children.
	if node.Function == ast.FUNC_CONSTANT {
		return ast.NodeEvaluation{
			Index:       node.Index,
			Function:    node.Function,
			ReturnValue: node.Constant,
			Errors:      []error{},
		}, true
	}

	type nodeEvaluationResponse struct {
		Eval ast.NodeEvaluation
		Ok   bool
	}

	// Most nodes are cheaper to evaluate again than to hash and copy, so only cacheable nodes are
	// deduplicated across the evaluation.
	attrs, _ := node.Function.Attributes()
	cacheable := cache != nil && attrs.Cacheable

	var hash uint64

	if cacheable {
		hash = node.Hash()

		if cached, ok := cache.Cache.Load(hash); ok {
			response := cached.(nodeEvaluationResponse)

			if eval, err := response.Eval.Clone(); err == nil {
				eval.Index = node.Index

				eval.EvaluationPlan = ast.NodeEvaluationPlan{
					Took:   0,
					Cached: true,
				}

				return eval, response.Ok
			}
		}
	}

	childEvaluationFail := false

	evalChild := func(child ast.Node) (childEval ast.NodeEvaluation, evalNext bool) {
		childEval, ok := EvaluateAst(ctx, cache, environment, child)

		if !ok {
			childEvaluationFail = true
		}

		// Should we continue evaluating subsequent children nodes?
		// We always do if circuit breaking is disabled in the environment or if the parent node does not support lazy evaluation.
		// Otherwise, we run the lazy evaluator to determine if we should continue or stop.
		evalNext = environment.disableCircuitBreaking || attrs.LazyChildEvaluation == nil || attrs.LazyChildEvaluation(childEval)

		return
	}

	evaluated := false

	evaluate := func() (any, error) {
		evaluated = true
		weightedNodes := NewWeightedNodes(environment, node, node.Children)

		// Named children should aways be evaluated, regardless of circuit-breaking
		namedChildren := pure_utils.MapValues(node.NamedChildren, func(node ast.Node) ast.NodeEvaluation {
			v, _ := evalChild(node)
			return v
		})

		// eval each child
		evaluation := ast.NodeEvaluation{
			Index:         node.Index,
			Function:      node.Function,
			Children:      weightedNodes.Reorder(pure_utils.MapWhile(weightedNodes.Sorted(), evalChild)),
			NamedChildren: namedChildren,
		}

		if childEvaluationFail {
			// an error occurred in at least one of the children. Stop the evaluation.

			// the frontend expects an ErrUndefinedFunction error to be present even when no evaluation happened.
			if node.Function == ast.FUNC_UNDEFINED {
				evaluation.Errors = append(evaluation.Errors, ast.ErrUndefinedFunction)
			}

			return nodeEvaluationResponse{evaluation, false}, nil
		}

		getReturnValue := func(e ast.NodeEvaluation) any { return e.ReturnValue }
		arguments := ast.Arguments{
			Args:      pure_utils.Map(evaluation.Children, getReturnValue),
			NamedArgs: pure_utils.MapValues(evaluation.NamedChildren, getReturnValue),
		}

		evaluator, err := environment.GetEvaluator(node.Function)
		if err != nil {
			evaluation.Errors = append(evaluation.Errors, err)
			return nodeEvaluationResponse{evaluation, false}, nil
		}

		evaluation.ReturnValue, evaluation.Errors = evaluator.Evaluate(ctx, arguments)

		if evaluation.Errors == nil {
			// Assign an empty array to indicate that the evaluation occured.
			// The evaluator is not supposed to return a nil array of errors, but let's be nice.
			evaluation.Errors = []error{}
		}

		ok := len(evaluation.Errors) == 0

		if !ok {
			// The evaluator is supposed to return nil ReturnValue when an error is present.
			evaluation.ReturnValue = nil
		}

		evaluationResponse := nodeEvaluationResponse{evaluation, ok}

		if cacheable {
			cache.Cache.Store(hash, evaluationResponse)
		}

		return evaluationResponse, nil
	}

	var eval any

	if cacheable {
		eval, _, _ = cache.Executor.Do(fmt.Sprintf("%d", hash), evaluate)
	} else {
		eval, _ = evaluate()
	}

	evaluation := eval.(nodeEvaluationResponse)
	evaluation.Eval.Index = node.Index

	evaluation.Eval.EvaluationPlan = ast.NodeEvaluationPlan{
		Took: time.Since(start),
	}

	if !evaluated {
		result, err := evaluation.Eval.Clone()
		if err != nil {
			evaluation.Eval.Errors = append(evaluation.Eval.Errors, err)

			return evaluation.Eval, false
		}

		evaluation.Eval = result
		evaluation.Eval.SetCached()
	}

	return evaluation.Eval, evaluation.Ok
}
