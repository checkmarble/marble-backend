package evaluate_scenario

import (
	"context"
	"errors"
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

type panickingAstEvaluator struct{}

func (panickingAstEvaluator) EvaluateAstExpression(ctx context.Context, cache *ast_eval.EvaluationCache, ruleAstExpression ast.Node, organizationId uuid.UUID, payload models.ClientObject, dataModel models.DataModel) (ast.NodeEvaluation, error) {
	panic("boom")
}

// Regression test: a panic in a rule goroutine used to be unrecoverable by EvalScenario and
// crashed the whole worker process.
func TestEvalAllScenarioRulesRecoversFromPanic(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), slog.New(slog.DiscardHandler))
	evaluator := ScenarioEvaluator{evaluateAstExpression: panickingAstEvaluator{}}

	rules := []models.Rule{
		{Id: "rule_1", FormulaAstExpression: &ast.Node{Constant: true}},
		{Id: "rule_2", FormulaAstExpression: &ast.Node{Constant: true}},
	}

	_, _, err := evaluator.evalAllScenarioRules(ctx, ast_eval.NewEvaluationCache(), rules,
		DataAccessor{}, models.DataModel{}, nil, 0)

	require.ErrorIs(t, err, models.ErrPanicInScenarioEvalution)
}

func TestScreeningRecoversFromPanic(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), slog.New(slog.DiscardHandler))
	evaluator := ScenarioEvaluator{evaluateAstExpression: panickingAstEvaluator{}}
	iteration := models.ScenarioIteration{ScreeningConfigs: []models.ScreeningConfig{
		{TriggerRule: &ast.Node{Constant: true}},
	}}

	_, err := evaluator.evaluateScreening(ctx, iteration, ScenarioEvaluationParameters{}, DataAccessor{})
	require.ErrorIs(t, err, models.ErrPanicInScenarioEvalution)
}

func TestScreeningRecoversFromConcurrentPanics(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), slog.New(slog.DiscardHandler))
	evaluator := ScenarioEvaluator{evaluateAstExpression: panickingAstEvaluator{}}
	configs := make([]models.ScreeningConfig, 20)
	for i := range configs {
		configs[i].TriggerRule = &ast.Node{Constant: true}
	}

	_, err := evaluator.evaluateScreening(ctx, models.ScenarioIteration{ScreeningConfigs: configs}, ScenarioEvaluationParameters{}, DataAccessor{})
	require.ErrorIs(t, err, models.ErrPanicInScenarioEvalution)
}

type mixedFailureAstEvaluator struct{}

func (mixedFailureAstEvaluator) EvaluateAstExpression(ctx context.Context, cache *ast_eval.EvaluationCache, node ast.Node, organizationID uuid.UUID, payload models.ClientObject, dataModel models.DataModel) (ast.NodeEvaluation, error) {
	if node.Constant == true {
		panic("boom")
	}
	return ast.NodeEvaluation{}, errors.New("screening failure")
}

func TestScreeningPanicTakesPrecedenceOverOtherErrors(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), slog.New(slog.DiscardHandler))
	evaluator := ScenarioEvaluator{evaluateAstExpression: mixedFailureAstEvaluator{}}
	iteration := models.ScenarioIteration{ScreeningConfigs: []models.ScreeningConfig{
		{TriggerRule: &ast.Node{Constant: true}},
		{TriggerRule: &ast.Node{Constant: false}},
	}}

	_, err := evaluator.evaluateScreening(ctx, iteration, ScenarioEvaluationParameters{}, DataAccessor{})
	require.ErrorIs(t, err, models.ErrPanicInScenarioEvalution)
}
