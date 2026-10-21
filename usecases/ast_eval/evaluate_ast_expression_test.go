package ast_eval

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func countingEnvironmentFactory(builds *atomic.Int64) AstEvaluationEnvironmentFactory {
	return func(params EvaluationEnvironmentFactoryParams) AstEvaluationEnvironment {
		builds.Add(1)
		return NewAstEvaluationEnvironment()
	}
}

func TestEvaluateAstExpressionSharesEnvironmentPerCache(t *testing.T) {
	var builds atomic.Int64

	evaluator := EvaluateAstExpression{AstEvaluationEnvironmentFactory: countingEnvironmentFactory(&builds)}
	expression := ast.Node{Function: ast.FUNC_EQUAL}.
		AddChild(ast.Node{Constant: 1}).
		AddChild(ast.Node{Constant: 1})

	cache := NewEvaluationCache()

	var wg sync.WaitGroup

	for range 20 {
		wg.Go(func() {
			evaluation, err := evaluator.EvaluateAstExpression(context.TODO(), cache, expression,
				uuid.New(), models.ClientObject{}, models.DataModel{})

			require.NoError(t, err)
			assert.Equal(t, true, evaluation.ReturnValue)
		})
	}

	wg.Wait()

	assert.Equal(t, int64(1), builds.Load())
}

func TestEvaluateAstExpressionWithoutCacheBuildsEnvironment(t *testing.T) {
	var builds atomic.Int64

	evaluator := EvaluateAstExpression{AstEvaluationEnvironmentFactory: countingEnvironmentFactory(&builds)}
	expression := ast.Node{Constant: true}

	for range 3 {
		_, err := evaluator.EvaluateAstExpression(context.TODO(), nil, expression,
			uuid.New(), models.ClientObject{}, models.DataModel{})
		require.NoError(t, err)
	}

	assert.Equal(t, int64(3), builds.Load())
}
