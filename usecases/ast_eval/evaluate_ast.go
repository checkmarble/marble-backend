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
		eval ast.NodeEvaluation
		ok   bool
	}

	hash := node.Hash()

	if cache != nil {
		if cached, ok := cache.Cache.Load(hash); ok {
			response := cached.(nodeEvaluationResponse)
			response.eval.Index = node.Index

			response.eval.EvaluationPlan = ast.NodeEvaluationPlan{
				Took:   0,
				Cached: true,
			}

			return response.eval, response.ok
		}
	}

	childEvaluationFail := false

	// Only interested in lazy callback which will have default value if an error is returned
	attrs, _ := node.Function.Attributes()

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

	cachedExecutor := new(singleflight.Group)
	notCached := false

	if cache != nil {
		cachedExecutor = cache.Executor
	}

	eval, _, _ := cachedExecutor.Do(fmt.Sprintf("%d", hash), func() (any, error) {
		notCached = true
		weightedNodes := NewWeightedNodes(environment, node, node.Children)

		// eval each child
		evaluation := ast.NodeEvaluation{
			Index:         node.Index,
			Function:      node.Function,
			Children:      weightedNodes.Reorder(pure_utils.MapWhile(weightedNodes.Sorted(), evalChild)),
			NamedChildren: pure_utils.MapValuesWhile(node.NamedChildren, evalChild),
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

		if cache != nil {
			cache.Cache.Store(hash, evaluationResponse)
		}

		return evaluationResponse, nil
	})

	evaluation := eval.(nodeEvaluationResponse)
	evaluation.eval.Index = node.Index

	evaluation.eval.EvaluationPlan = ast.NodeEvaluationPlan{
		Took: time.Since(start),
	}

	if !notCached {
		evaluation.eval.SetCached()
	}

	return evaluation.eval, evaluation.ok
}
