package evaluate_scenario

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type alwaysHitAstEvaluator struct{}

func (alwaysHitAstEvaluator) EvaluateAstExpression(ctx context.Context, cache *ast_eval.EvaluationCache, ruleAstExpression ast.Node, organizationId uuid.UUID, payload models.ClientObject, dataModel models.DataModel) (ast.NodeEvaluation, error) {
	return ast.NodeEvaluation{ReturnValue: true, Errors: []error{}}, nil
}

// Regression test for potential race condition when summing concurrent rule execution score modifiers.
func TestEvalAllScenarioRulesSumsEveryScore(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), slog.New(slog.DiscardHandler))
	evaluator := ScenarioEvaluator{evaluateAstExpression: alwaysHitAstEvaluator{}}

	rules := make([]models.Rule, 500)
	for i := range rules {
		rules[i] = models.Rule{
			Id:                   fmt.Sprintf("rule_%d", i),
			FormulaAstExpression: &ast.Node{Constant: true},
			ScoreModifier:        1,
		}
	}

	for range 200 {
		score, executions, err := evaluator.evalAllScenarioRules(ctx, ast_eval.NewEvaluationCache(), rules,
			DataAccessor{}, models.DataModel{}, nil, 0)

		require.NoError(t, err)
		require.Len(t, executions, len(rules))
		assert.Equal(t, len(rules), score)
	}
}
