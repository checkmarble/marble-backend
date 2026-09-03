package ast_eval

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScoreComputation(t *testing.T) {
	environment := NewAstEvaluationEnvironment()

	root := ast.Node{
		Function: ast.FUNC_SCORE_COMPUTATION,
		Children: []ast.Node{
			{
				Function: ast.FUNC_EQUAL,
				Children: []ast.Node{{Constant: 1}, {Constant: 1}},
			},
		},
		NamedChildren: map[string]ast.Node{
			"modifier": {Constant: 100},
			"floor":    {Constant: 3},
		},
	}

	evaluation, ok := EvaluateAst(context.TODO(), nil, environment, root)
	assert.True(t, ok)
	assert.NotNil(t, evaluation.ReturnValue)

	scoring, ok := evaluation.ReturnValue.(ast.ScoreComputationResult)
	assert.True(t, ok)

	assert.Equal(t, 100, scoring.Modifier)
	assert.Equal(t, 3, scoring.Floor)
}

func TestScoreComputationNotTriggered(t *testing.T) {
	environment := NewAstEvaluationEnvironment()

	root := ast.Node{
		Function: ast.FUNC_SCORE_COMPUTATION,
		Children: []ast.Node{
			{
				Function: ast.FUNC_EQUAL,
				Children: []ast.Node{{Constant: 1}, {Constant: 3}},
			},
		},
		NamedChildren: map[string]ast.Node{
			"modifier": {Constant: 100},
			"floor":    {Constant: 3},
		},
	}

	evaluation, ok := EvaluateAst(context.TODO(), nil, environment, root)
	assert.True(t, ok)

	assert.Equal(t, ast.ScoreComputationResult{}, evaluation.ReturnValue)
}

func TestSwitchScoring(t *testing.T) {
	environment := NewAstEvaluationEnvironment()

	root := ast.Node{
		Function: ast.FUNC_SWITCH,
		Children: []ast.Node{
			{
				Function: ast.FUNC_SCORE_COMPUTATION,
				Children: []ast.Node{
					{
						Function: ast.FUNC_EQUAL,
						Children: []ast.Node{{Constant: 1}, {Constant: 3}},
					},
				},
				NamedChildren: map[string]ast.Node{
					"modifier": {Constant: -50},
					"floor":    {Constant: 0},
				},
			},
			{
				Function: ast.FUNC_SCORE_COMPUTATION,
				Children: []ast.Node{
					{
						Function: ast.FUNC_EQUAL,
						Children: []ast.Node{{Constant: 1}, {Constant: 1}},
					},
				},
				NamedChildren: map[string]ast.Node{
					"modifier": {Constant: 100},
					"floor":    {Constant: 3},
				},
			},
			{
				Function: ast.FUNC_SCORE_COMPUTATION,
				Children: []ast.Node{
					{
						Function: ast.FUNC_EQUAL,
						Children: []ast.Node{{Constant: 1}, {Constant: 2}},
					},
				},
				NamedChildren: map[string]ast.Node{
					"modifier": {Constant: -30},
					"floor":    {Constant: 1},
				},
			},
		},
		NamedChildren: map[string]ast.Node{
			"field": {Constant: "Value"},
		},
	}

	evaluation, ok := EvaluateAst(context.TODO(), nil, environment, root)
	assert.True(t, ok)
	assert.NotNil(t, evaluation.ReturnValue)

	assert.Len(t, evaluation.Children, 2) // Only two children evaluated, last one is skipped

	scoring, ok := evaluation.ReturnValue.(ast.ScoreComputationResult)
	assert.True(t, ok)

	assert.Equal(t, 100, scoring.Modifier)
	assert.Equal(t, 3, scoring.Floor)
	require.NotNil(t, scoring.Branch)
	assert.Equal(t, 1, *scoring.Branch)
	assert.False(t, scoring.Fallback)
	assert.False(t, scoring.Default)
}

func TestSwitchScoringDefaultCase(t *testing.T) {
	environment := NewAstEvaluationEnvironment()

	root := ast.Node{
		Function: ast.FUNC_SWITCH,
		Children: []ast.Node{
			{
				Function: ast.FUNC_SCORE_COMPUTATION,
				Children: []ast.Node{
					{
						Function: ast.FUNC_EQUAL,
						Children: []ast.Node{{Constant: 1}, {Constant: 3}},
					},
				},
				NamedChildren: map[string]ast.Node{
					"modifier": {Constant: -50},
					"floor":    {Constant: 0},
				},
			},
			{
				Function: ast.FUNC_SCORE_COMPUTATION,
				Children: []ast.Node{
					{
						Function: ast.FUNC_EQUAL,
						Children: []ast.Node{{Constant: 1}, {Constant: 7}},
					},
				},
				NamedChildren: map[string]ast.Node{
					"modifier": {Constant: 100},
					"floor":    {Constant: 3},
				},
			},
			{
				Function: ast.FUNC_SCORE_COMPUTATION,
				Children: []ast.Node{{Constant: true}},
				NamedChildren: map[string]ast.Node{
					"modifier": {Constant: 42},
					"floor":    {Constant: 9000},
				},
			},
		},
		NamedChildren: map[string]ast.Node{
			"field": {Constant: "Value"},
		},
	}

	evaluation, ok := EvaluateAst(context.TODO(), nil, environment, root)
	assert.True(t, ok)
	assert.NotNil(t, evaluation.ReturnValue)

	scoring, ok := evaluation.ReturnValue.(ast.ScoreComputationResult)
	assert.True(t, ok)

	assert.Equal(t, 42, scoring.Modifier)
	assert.Equal(t, 9000, scoring.Floor)
	require.NotNil(t, scoring.Branch)
	assert.Equal(t, 2, *scoring.Branch)
	assert.False(t, scoring.Fallback)
	assert.False(t, scoring.Default)
}

func TestSwitchScoringFallbackMetadata(t *testing.T) {
	environment := NewAstEvaluationEnvironment()
	root := ast.Node{Function: ast.FUNC_SWITCH}.
		AddChild(ast.Node{Function: ast.FUNC_SCORE_COMPUTATION}.
			AddChild(ast.NewNodeConstant(false)).
			AddNamedChild("modifier", ast.NewNodeConstant(10))).
		AddNamedChild("fallback", ast.Node{Function: ast.FUNC_SCORE_COMPUTATION}.
			AddChild(ast.NewNodeConstant(true)).
			AddNamedChild("modifier", ast.NewNodeConstant(25)))

	evaluation, ok := EvaluateAst(context.Background(), nil, environment, root)

	require.True(t, ok)
	scoring, ok := evaluation.ReturnValue.(ast.ScoreComputationResult)
	require.True(t, ok)
	assert.Equal(t, 25, scoring.Modifier)
	assert.Nil(t, scoring.Branch)
	assert.True(t, scoring.Fallback)
	assert.False(t, scoring.Default)
}

func TestSwitchScoringDefaultMetadata(t *testing.T) {
	environment := NewAstEvaluationEnvironment()
	root := ast.Node{Function: ast.FUNC_SWITCH}.
		AddChild(ast.Node{Function: ast.FUNC_SCORE_COMPUTATION}.
			AddChild(ast.NewNodeConstant(false)).
			AddNamedChild("modifier", ast.NewNodeConstant(10)))

	evaluation, ok := EvaluateAst(context.Background(), nil, environment, root)

	require.True(t, ok)
	scoring, ok := evaluation.ReturnValue.(ast.ScoreComputationResult)
	require.True(t, ok)
	assert.True(t, scoring.Triggered)
	assert.Nil(t, scoring.Branch)
	assert.False(t, scoring.Fallback)
	assert.True(t, scoring.Default)
}

func TestSwitchErrorOnNoCaseExecuted(t *testing.T) {
	environment := NewAstEvaluationEnvironment()

	root := ast.Node{
		Function: ast.FUNC_SWITCH,
		Children: []ast.Node{
			{
				Function: ast.FUNC_SCORE_COMPUTATION,
				Children: []ast.Node{},
				NamedChildren: map[string]ast.Node{
					"modifier": {Constant: 42},
					"floor":    {Constant: 9000},
				},
			},
		},
	}

	_, ok := EvaluateAst(context.TODO(), nil, environment, root)

	assert.False(t, ok)
}
