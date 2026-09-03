package ast_eval

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNumericSwitchUsesFirstMatchingCase(t *testing.T) {
	environment := NewAstEvaluationEnvironment()
	root := numericSwitchNode(
		10,
		numericCaseNode(ast.NewNodeConstant(false), 20),
		numericCaseNode(ast.NewNodeConstant(true), 30),
		ast.Node{
			Function: ast.FUNC_CASE,
			Children: []ast.Node{
				{Function: ast.FUNC_UNDEFINED},
				ast.NewNodeConstant(40),
			},
		},
	)

	evaluation, ok := EvaluateAst(context.Background(), nil, environment, root)

	require.True(t, ok)
	assert.Equal(t, 30.0, evaluation.ReturnValue)
	assert.Len(t, evaluation.Children, 2)
}

func TestNumericSwitchUsesRequiredFallback(t *testing.T) {
	environment := NewAstEvaluationEnvironment()

	t.Run("returns fallback when no case matches", func(t *testing.T) {
		root := numericSwitchNode(12, numericCaseNode(ast.NewNodeConstant(false), 20))

		evaluation, ok := EvaluateAst(context.Background(), nil, environment, root)

		require.True(t, ok)
		assert.Equal(t, 12.0, evaluation.ReturnValue)
	})

	t.Run("rejects a missing fallback", func(t *testing.T) {
		root := ast.Node{Function: ast.FUNC_SWITCH}.
			AddChild(numericCaseNode(ast.NewNodeConstant(false), 20))

		evaluation, ok := EvaluateAst(context.Background(), nil, environment, root)

		require.False(t, ok)
		require.Len(t, evaluation.Errors, 1)
		assert.ErrorIs(t, evaluation.Errors[0], ast.ErrMissingNamedArgument)
		var argumentErr ast.ArgumentError
		require.ErrorAs(t, evaluation.Errors[0], &argumentErr)
		assert.Equal(t, ast.NewNamedArgumentError("fallback"), argumentErr)
	})

	t.Run("rejects a non-numeric fallback", func(t *testing.T) {
		root := numericSwitchNode("default", numericCaseNode(ast.NewNodeConstant(false), 20))

		evaluation, ok := EvaluateAst(context.Background(), nil, environment, root)

		require.False(t, ok)
		require.Len(t, evaluation.Errors, 1)
		assert.ErrorIs(t, evaluation.Errors[0], ast.ErrArgumentMustBeIntOrFloat)
	})
}

func TestNumericSwitchSupportsStringFieldPredicate(t *testing.T) {
	environment := NewAstEvaluationEnvironment()
	environment.AddEvaluator(ast.FUNC_PAYLOAD, evaluate.NewPayload(ast.FUNC_PAYLOAD, models.ClientObject{
		Data: map[string]any{"status": "PEP"},
	}))
	predicate := ast.Node{Function: ast.FUNC_EQUAL}.
		AddChild(ast.Node{Function: ast.FUNC_PAYLOAD}.AddChild(ast.NewNodeConstant("status"))).
		AddChild(ast.NewNodeConstant("PEP"))
	root := numericSwitchNode(5, numericCaseNode(predicate, 25))

	evaluation, ok := EvaluateAst(context.Background(), nil, environment, root)

	require.True(t, ok)
	assert.Equal(t, 25.0, evaluation.ReturnValue)
}

func TestNumericSwitchSupportsTwoVariablePredicate(t *testing.T) {
	environment := NewAstEvaluationEnvironment()
	environment.AddEvaluator(ast.FUNC_PAYLOAD, evaluate.NewPayload(ast.FUNC_PAYLOAD, models.ClientObject{
		Data: map[string]any{"status": "PEP"},
	}))
	environment.AddEvaluator(ast.FUNC_RECORD_RISK_LEVEL, staticEvaluator{result: true})

	stringPredicate := ast.Node{Function: ast.FUNC_EQUAL}.
		AddChild(ast.Node{Function: ast.FUNC_PAYLOAD}.AddChild(ast.NewNodeConstant("status"))).
		AddChild(ast.NewNodeConstant("PEP"))
	riskLevelPredicate := ast.Node{Function: ast.FUNC_RECORD_RISK_LEVEL}.
		AddChild(ast.NewNodeConstant([]float64{2}))
	combinedPredicate := ast.Node{Function: ast.FUNC_AND}.
		AddChild(stringPredicate).
		AddChild(riskLevelPredicate)
	root := numericSwitchNode(5, numericCaseNode(combinedPredicate, 35))

	evaluation, ok := EvaluateAst(context.Background(), nil, environment, root)

	require.True(t, ok)
	assert.Equal(t, 35.0, evaluation.ReturnValue)
}

func TestSwitchRejectsMixedBranchKindsDuringValidation(t *testing.T) {
	environment := NewAstEvaluationEnvironment().WithoutOptimizations()
	scoringCase := ast.Node{Function: ast.FUNC_SCORE_COMPUTATION}.
		AddChild(ast.NewNodeConstant(true)).
		AddNamedChild("modifier", ast.NewNodeConstant(10))
	root := numericSwitchNode(
		5,
		numericCaseNode(ast.NewNodeConstant(false), 20),
		scoringCase,
	)

	evaluation, ok := EvaluateAst(context.Background(), nil, environment, root)

	require.False(t, ok)
	require.Len(t, evaluation.Errors, 1)
	assert.ErrorIs(t, evaluation.Errors[0], ast.ErrArgumentInvalidType)
	var argumentErr ast.ArgumentError
	require.ErrorAs(t, evaluation.Errors[0], &argumentErr)
	assert.Equal(t, ast.NewArgumentError(1), argumentErr)
}

type staticEvaluator struct {
	result any
}

func (e staticEvaluator) Evaluate(context.Context, ast.Arguments) (any, []error) {
	return e.result, nil
}

func numericCaseNode(predicate ast.Node, value any) ast.Node {
	return ast.Node{Function: ast.FUNC_CASE}.
		AddChild(predicate).
		AddChild(ast.NewNodeConstant(value))
}

func numericSwitchNode(fallback any, cases ...ast.Node) ast.Node {
	root := ast.Node{Function: ast.FUNC_SWITCH, Children: cases}
	return root.AddNamedChild("fallback", ast.NewNodeConstant(fallback))
}
