package ast_eval

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/stretchr/testify/assert"
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
			"default": {
				Function: ast.FUNC_SCORE_COMPUTATION,
				Children: []ast.Node{{Constant: true}},
				NamedChildren: map[string]ast.Node{
					"modifier": {Constant: 42},
					"floor":    {Constant: 9000},
				},
			},
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
	}

	evaluation, ok := EvaluateAst(context.TODO(), nil, environment, root)
	assert.True(t, ok)
	assert.NotNil(t, evaluation.ReturnValue)

	scoring, ok := evaluation.ReturnValue.(ast.ScoreComputationResult)
	assert.True(t, ok)

	assert.Equal(t, 42, scoring.Modifier)
	assert.Equal(t, 9000, scoring.Floor)
}

func TestSwitchErrorOnNoCaseExecuted(t *testing.T) {
	environment := NewAstEvaluationEnvironment()

	root := ast.Node{
		Function: ast.FUNC_SWITCH,
		Children: []ast.Node{
			{
				Function: ast.FUNC_SCORE_COMPUTATION,
				Children: []ast.Node{{Constant: false}},
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
