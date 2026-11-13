package evaluate_scenario

import (
	"context"
	"log/slog"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
			iteration := models.ScenarioIteration{
				Id: testID, Version: new(2), ScoreReviewThreshold: new(10),
				ScoreBlockAndReviewThreshold: new(20), ScoreDeclineThreshold: new(30),
				Rules: []models.Rule{{Id: "rule", FormulaAstExpression: &ast.Node{Constant: true}, ScoreModifier: tt.score}},
			}
			executor := &mocks.Executor{}
			iterations := &mocks.ScenarioIterationReadRepository{}
			iterations.On("GetScenarioIteration", mock.Anything, executor, testID, true).Return(iteration, nil)
			configs := &mocks.ScreeningConfigRepository{}
			configs.On("ListScreeningConfigs", mock.Anything, executor, testID, true).
				Return([]models.ScreeningConfig{testConfig, secondTestConfig}, nil)
			configs.On("ListScreeningConfigs", mock.Anything, executor, liveID, true).
				Return([]models.ScreeningConfig{liveConfig, secondLiveConfig}, nil)
			factory := &mocks.ExecutorFactory{}
			factory.On("NewExecutor").Return(executor)
			testRuns := &mocks.ScenarioTestrunRepository{}
			testRuns.On("ListTestRunsByScenarioID", mock.Anything, executor, scenario.Id, models.Up).
				Return([]models.ScenarioTestRun{{Id: "test-run", Status: models.Up, ScenarioIterationId: testID, ScenarioLiveIterationId: liveID}}, nil)
			evaluator := ScenarioEvaluator{
				executorFactory: factory, scenarioTestRunRepository: testRuns,
				evalScenarioRepository: iterations, evalScreeningConfigRepository: configs,
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

func TestEvalTestRunScenarioReplaysChangedScreeningBeforeCachedScreening(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), slog.New(slog.DiscardHandler))
	liveID, testID := uuid.NewString(), uuid.NewString()
	scenario := models.Scenario{Id: uuid.NewString(), LiveVersionID: &liveID, TriggerObjectType: "transactions"}
	liveA := models.ScreeningConfig{
		Id: "live-A", StableId: "A", ScenarioIterationId: liveID, ForcedOutcome: models.Decline,
		Query: map[string]ast.Node{"name": {Constant: "old A"}},
	}
	testA := liveA
	testA.Id, testA.ScenarioIterationId = "test-A", testID
	testA.Query = map[string]ast.Node{"name": {Constant: "new A"}}
	liveB := models.ScreeningConfig{
		Id: "live-B", StableId: "B", ScenarioIterationId: liveID, ForcedOutcome: models.Review,
		Query: map[string]ast.Node{"name": {Constant: "B"}},
	}
	testB := liveB
	testB.Id, testB.ScenarioIterationId = "test-B", testID
	iteration := models.ScenarioIteration{
		Id: testID, Version: new(2), ScoreReviewThreshold: new(10),
		ScoreBlockAndReviewThreshold: new(20), ScoreDeclineThreshold: new(30),
	}
	executor := &mocks.Executor{}
	iterations := &mocks.ScenarioIterationReadRepository{}
	iterations.On("GetScenarioIteration", mock.Anything, executor, testID, true).Return(iteration, nil)
	configs := &mocks.ScreeningConfigRepository{}
	configs.On("ListScreeningConfigs", mock.Anything, executor, testID, true).
		Return([]models.ScreeningConfig{testA, testB}, nil)
	configs.On("ListScreeningConfigs", mock.Anything, executor, liveID, true).
		Return([]models.ScreeningConfig{liveA, liveB}, nil)
	factory := &mocks.ExecutorFactory{}
	factory.On("NewExecutor").Return(executor)
	testRuns := &mocks.ScenarioTestrunRepository{}
	testRuns.On("ListTestRunsByScenarioID", mock.Anything, executor, scenario.Id, models.Up).
		Return([]models.ScenarioTestRun{{Id: "test-run", Status: models.Up, ScenarioIterationId: testID, ScenarioLiveIterationId: liveID}}, nil)
	evaluator, screeningExecutor := getScreeningEvaluator()
	screeningExecutor.Mock.On("Execute", mock.Anything, scenario.OrganizationId, mock.Anything).Return().Once()
	evaluator.executorFactory = factory
	evaluator.scenarioTestRunRepository = testRuns
	evaluator.evalScenarioRepository = iterations
	evaluator.evalScreeningConfigRepository = configs
	cachedA := models.ScreeningWithMatches{Screening: models.Screening{
		Id: "cached-A", ScreeningConfigId: liveA.Id, Status: models.ScreeningStatusNoHit,
	}}
	cachedB := models.ScreeningWithMatches{Screening: models.Screening{
		Id: "cached-B", ScreeningConfigId: liveB.Id, Status: models.ScreeningStatusInReview, NumberOfMatches: 1,
	}}

	triggered, result, err := evaluator.EvalTestRunScenario(ctx, ScenarioEvaluationParameters{
		Scenario:     scenario,
		ClientObject: models.ClientObject{TableName: "transactions", Data: map[string]any{"object_id": "transaction"}},
		CachedScreenings: map[string]models.ScreeningWithMatches{
			liveA.Id: cachedA, liveB.Id: cachedB,
		},
	})

	require.NoError(t, err)
	require.True(t, triggered)
	require.Len(t, result.ScreeningExecutions, 2)
	assert.Equal(t, testA.Id, result.ScreeningExecutions[0].ScreeningConfigId)
	assert.NotEmpty(t, result.ScreeningExecutions[0].Id)
	assert.NotEqual(t, cachedA.Id, result.ScreeningExecutions[0].Id)
	assert.Equal(t, cachedB, result.ScreeningExecutions[1])
	assert.Equal(t, models.Review, result.Outcome)
	screeningExecutor.Mock.AssertNumberOfCalls(t, "Execute", 1)
}
