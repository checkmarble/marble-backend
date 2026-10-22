package decision_workflows

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayloadEvaluates(t *testing.T) {
	req := DecisionWorkflowRequest{
		EvaluateAst: ast_eval.EvaluateAstExpression{
			AstEvaluationEnvironmentFactory: func(ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
				return ast_eval.NewAstEvaluationEnvironment()
			},
		},
	}

	t.Run("null result is not a match", func(t *testing.T) {
		node := ast.Node{Function: ast.FUNC_AND}.
			AddChild(ast.Node{Constant: true}).
			AddChild(ast.Node{Constant: nil})

		match, err := payloadEvaluates(node)(context.TODO(), req)

		require.NoError(t, err)
		assert.False(t, match)
	})

	t.Run("true result is a match", func(t *testing.T) {
		node := ast.Node{Function: ast.FUNC_AND}.
			AddChild(ast.Node{Constant: true}).
			AddChild(ast.Node{Constant: true})

		match, err := payloadEvaluates(node)(context.TODO(), req)

		require.NoError(t, err)
		assert.True(t, match)
	})

	t.Run("non-boolean result is an error", func(t *testing.T) {
		_, err := payloadEvaluates(ast.Node{Constant: 42})(context.TODO(), req)

		assert.ErrorIs(t, err, ast.ErrArgumentMustBeBool)
	})
}
