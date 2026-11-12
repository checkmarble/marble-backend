package evaluate_scenario

import (
	"context"
	"log/slog"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type cachedScreeningScenarioRepository struct {
	iteration models.ScenarioIteration
	configs   map[string][]models.ScreeningConfig
}

func (r cachedScreeningScenarioRepository) GetScenarioIteration(context.Context, repositories.Executor, string, bool) (models.ScenarioIteration, error) {
	return r.iteration, nil
}

func (r cachedScreeningScenarioRepository) ListScreeningConfigs(_ context.Context, _ repositories.Executor, iterationID string, _ bool) ([]models.ScreeningConfig, error) {
	return r.configs[iterationID], nil
}

func TestEvalTestRunScenarioUsesCachedScreeningForOutcome(t *testing.T) {
	for _, tt := range []struct {
		name    string
		matches int
		score   int
		want    models.Outcome
	}{
		{name: "cached hit forces review", matches: 1, want: models.Review},
		{name: "cached hit overrides nominal decline", matches: 1, score: 100, want: models.Review},
		{name: "cached no hit uses nominal score", score: 100, want: models.Decline},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := utils.StoreLoggerInContext(context.Background(), slog.New(slog.DiscardHandler))
			liveID, testID := uuid.NewString(), uuid.NewString()
			scenario := models.Scenario{Id: uuid.NewString(), LiveVersionID: &liveID, TriggerObjectType: "transactions"}
			liveConfig := models.ScreeningConfig{Id: "live-config", StableId: "screening", Threshold: new(70), ForcedOutcome: models.Review}
			testConfig := liveConfig
			liveConfig.ScenarioIterationId = liveID
			testConfig.Id, testConfig.ScenarioIterationId, testConfig.Threshold = "test-config", testID, new(70)
			secondLiveConfig := models.ScreeningConfig{
				Id: "second-live-config", StableId: "second-screening", ScenarioIterationId: liveID, ForcedOutcome: models.Decline,
			}
			secondTestConfig := secondLiveConfig
			secondTestConfig.Id, secondTestConfig.ScenarioIterationId = "second-test-config", testID
			repo := cachedScreeningScenarioRepository{
				iteration: models.ScenarioIteration{
					Id: testID, Version: new(2), ScoreReviewThreshold: new(10),
					ScoreBlockAndReviewThreshold: new(20), ScoreDeclineThreshold: new(30),
					Rules: []models.Rule{{Id: "rule", FormulaAstExpression: &ast.Node{Constant: true}, ScoreModifier: tt.score}},
				},
				configs: map[string][]models.ScreeningConfig{
					liveID: {liveConfig, secondLiveConfig}, testID: {testConfig, secondTestConfig},
				},
			}
			executor := &mocks.Executor{}
			factory := &mocks.ExecutorFactory{}
			factory.On("NewExecutor").Return(executor)
			testRuns := &mocks.ScenarioTestrunRepository{}
			testRuns.On("ListTestRunsByScenarioID", mock.Anything, executor, scenario.Id, models.Up).
				Return([]models.ScenarioTestRun{{Id: "test-run", Status: models.Up, ScenarioIterationId: testID, ScenarioLiveIterationId: liveID}}, nil)
			evaluator := ScenarioEvaluator{
				executorFactory: factory, scenarioTestRunRepository: testRuns,
				evalScenarioRepository: repo, evalScreeningConfigRepository: repo,
				evaluateAstExpression: alwaysHitAstEvaluator{},
			}
			status := models.ScreeningStatusNoHit
			if tt.matches > 0 {
				status = models.ScreeningStatusInReview
			}
			firstScreening := models.ScreeningWithMatches{Screening: models.Screening{
				Id: "live-screening", ScreeningConfigId: liveConfig.Id, Status: status, NumberOfMatches: tt.matches,
			}}
			secondScreening := models.ScreeningWithMatches{Screening: models.Screening{
				Id: "second-live-screening", ScreeningConfigId: secondLiveConfig.Id, Status: models.ScreeningStatusNoHit,
			}}

			triggered, result, err := evaluator.EvalTestRunScenario(ctx, ScenarioEvaluationParameters{
				Scenario:     scenario,
				ClientObject: models.ClientObject{TableName: "transactions", Data: map[string]any{"object_id": "transaction"}},
				CachedScreenings: map[string]models.ScreeningWithMatches{
					liveConfig.Id: firstScreening, secondLiveConfig.Id: secondScreening,
				},
			})

			require.NoError(t, err)
			require.True(t, triggered)
			assert.Equal(t, []models.ScreeningWithMatches{firstScreening, secondScreening}, result.ScreeningExecutions)
			assert.Equal(t, tt.want, result.Outcome)
			assert.Equal(t, "test-run", result.TestRunId)
		})
	}
}
