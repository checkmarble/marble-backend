package ast_eval

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/stretchr/testify/assert"
)

func TestEval(t *testing.T) {
	environment := NewAstEvaluationEnvironment()
	root := ast.NewAstCompareBalance()
	evaluation, ok := EvaluateAst(context.TODO(), environment, root)
	assert.True(t, ok)
	assert.Len(t, evaluation.Errors, 0)
	assert.Equal(t, true, evaluation.ReturnValue)
}

func TestEvalUndefinedFunction(t *testing.T) {
	environment := NewAstEvaluationEnvironment()
	root := ast.Node{Function: ast.FUNC_UNDEFINED}
	evaluation, ok := EvaluateAst(context.TODO(), environment, root)
	assert.False(t, ok)
	if assert.Len(t, evaluation.Errors, 1) {
		assert.ErrorIs(t, evaluation.Errors[0], ast.ErrUndefinedFunction)
	}
}

func TestEvalAndOrFunction(t *testing.T) {
	environment := NewAstEvaluationEnvironment()

	evaluation, ok := EvaluateAst(context.TODO(), environment, NewAstAndTrue())
	assert.True(t, ok)
	assert.Len(t, evaluation.Errors, 0)
	assert.Equal(t, true, evaluation.ReturnValue)

	evaluation, ok = EvaluateAst(context.TODO(), environment, NewAstAndFalse())
	assert.True(t, ok)
	assert.Len(t, evaluation.Errors, 0)
	assert.Equal(t, false, evaluation.ReturnValue)

	evaluation, ok = EvaluateAst(context.TODO(), environment, NewAstOrTrue())
	assert.True(t, ok)
	assert.Len(t, evaluation.Errors, 0)
	assert.Equal(t, true, evaluation.ReturnValue)

	evaluation, ok = EvaluateAst(context.TODO(), environment, NewAstOrFalse())
	assert.True(t, ok)
	assert.Len(t, evaluation.Errors, 0)
	assert.Equal(t, false, evaluation.ReturnValue)
}

func NewAstAndTrue() ast.Node {
	return ast.Node{Function: ast.FUNC_AND}.
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true})
}

func NewAstAndFalse() ast.Node {
	return ast.Node{Function: ast.FUNC_AND}.
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true})
}

func NewAstOrTrue() ast.Node {
	return ast.Node{Function: ast.FUNC_OR}.
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false})
}

func NewAstOrFalse() ast.Node {
	return ast.Node{Function: ast.FUNC_OR}.
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false})
}

func TestLazyAnd(t *testing.T) {
	environment := NewAstEvaluationEnvironment().WithoutCostOptimizations()

	for _, value := range []bool{true, false} {
		root := ast.Node{Function: ast.FUNC_AND}.
			AddChild(ast.Node{Function: ast.FUNC_EQUAL}.
				AddChild(ast.Node{Constant: value}).
				AddChild(ast.Node{Constant: true})).
			AddChild(ast.Node{Function: ast.FUNC_UNKNOWN})

		evaluation, ok := EvaluateAst(context.TODO(), environment, root)

		switch value {
		case false:
			assert.True(t, ok, "unknown node should not be evaluated because of AND lazy evaluation")
			assert.Len(t, evaluation.Children, 1, "lazy evaluated AND should only have one child")
		case true:
			assert.False(t, ok, "unknown node should be evaluated because of AND lazy evaluation")
			assert.Len(t, evaluation.Children, 2, "lazy evaluated AND should have two children")
		}
	}
}

func TestLazyOr(t *testing.T) {
	environment := NewAstEvaluationEnvironment().WithoutCostOptimizations()

	for _, value := range []bool{true, false} {
		root := ast.Node{Function: ast.FUNC_OR}.
			AddChild(ast.Node{Function: ast.FUNC_EQUAL}.
				AddChild(ast.Node{Constant: value}).
				AddChild(ast.Node{Constant: true})).
			AddChild(ast.Node{Function: ast.FUNC_UNKNOWN})

		evaluation, ok := EvaluateAst(context.TODO(), environment, root)

		switch value {
		case true:
			assert.True(t, ok, "unknown node should not be evaluated because of OR lazy evaluation")
			assert.Len(t, evaluation.Children, 1, "lazy evaluates OR should only have one child")
		case false:
			assert.False(t, ok, "unknown node should be evaluated because of OR lazy evaluation")
			assert.Len(t, evaluation.Children, 2, "lazy evaluated AND should have two children")
		}
	}
}

func TestLazyBooleanNulls(t *testing.T) {
	tts := []struct {
		fn            ast.Function
		lhs, rhs, res *bool
	}{
		{ast.FUNC_OR, nil, utils.Ptr(true), utils.Ptr(true)},
		{ast.FUNC_OR, utils.Ptr(true), nil, utils.Ptr(true)},
		{ast.FUNC_OR, nil, utils.Ptr(false), nil},
		{ast.FUNC_OR, utils.Ptr(false), nil, nil},
		{ast.FUNC_AND, nil, utils.Ptr(true), nil},
		{ast.FUNC_AND, utils.Ptr(true), nil, nil},
		{ast.FUNC_AND, nil, utils.Ptr(false), utils.Ptr(false)},
		{ast.FUNC_AND, utils.Ptr(false), nil, utils.Ptr(false)},
	}

	environment := NewAstEvaluationEnvironment().WithoutCostOptimizations()

	for _, tt := range tts {
		root := ast.Node{Function: tt.fn}

		for _, op := range []*bool{tt.lhs, tt.rhs} {
			switch op {
			case nil:
				root = root.AddChild(ast.Node{Constant: nil})
			default:
				root = root.AddChild(ast.Node{Constant: *op})
			}
		}

		evaluation, _ := EvaluateAst(context.TODO(), environment, root)

		switch {
		case tt.res == nil:
			assert.Equal(t, nil, evaluation.ReturnValue)
		default:
			assert.Equal(t, *tt.res, evaluation.ReturnValue)
		}
	}
}

const TEST_FUNC_COSTLY = -10

type costlyNode struct{}

func (costlyNode) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	return evaluate.MakeEvaluateResult(false)
}

func TestAggregatesOrderedLast(t *testing.T) {
	ast.FuncAttributesMap[TEST_FUNC_COSTLY] = ast.FuncAttributes{
		Cost: 1000,
	}

	defer delete(ast.FuncAttributesMap, TEST_FUNC_COSTLY)

	environment := NewAstEvaluationEnvironment()
	environment.AddEvaluator(TEST_FUNC_COSTLY, costlyNode{})

	root := ast.Node{Function: ast.FUNC_OR}.
		AddChild(ast.Node{Function: TEST_FUNC_COSTLY}).
		AddChild(ast.Node{Constant: true})

	evaluation, ok := EvaluateAst(context.TODO(), environment, root)

	assert.True(t, ok)
	assert.Equal(t, ast.NodeEvaluation{Index: 0, Skipped: true, ReturnValue: nil}, evaluation.Children[0])
	assert.Equal(t, false, evaluation.Children[1].Skipped)
	assert.Equal(t, true, evaluation.Children[1].ReturnValue)
	assert.Equal(t, true, evaluation.ReturnValue)
}
