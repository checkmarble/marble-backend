package evaluate_scenario

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockScreeningExecutor struct {
	*mock.Mock
}

func (m mockScreeningExecutor) IsConfigured() bool {
	args := m.Called()

	return args.Bool(0)
}

func (m mockScreeningExecutor) Execute(
	ctx context.Context,
	orgId uuid.UUID,
	query models.OpenSanctionsQuery,
) (models.ScreeningWithMatches, error) {
	// We are not mocking returned data here, only that the function was called
	// with the appropriate arguments, so we always expect this to be called.
	m.On("Execute", context.TODO(), orgId, query)
	m.Called(ctx, orgId, query)

	return models.ScreeningWithMatches{}, nil
}

func (m mockScreeningExecutor) SearchWhitelist(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	counterparty, entityId *string,
	reviewId *models.UserId,
) ([]models.ScreeningWhitelist, error) {
	args := m.Called(ctx, exec, orgId, counterparty, entityId, reviewId)
	return args.Get(0).([]models.ScreeningWhitelist), args.Error(1)
}

func (m mockScreeningExecutor) PerformNameRecognition(ctx context.Context, label string) ([]httpmodels.HTTPNameRecognitionMatch, error) {
	m.On("PerformNameRecognition", ctx, label)
	args := m.Called(context.TODO(), label)

	return args.Get(0).([]httpmodels.HTTPNameRecognitionMatch), args.Error(1)
}

type customListAstMock struct{}

func (customListAstMock) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	return []string{"this", "forbidden"}, nil
}

func getScreeningEvaluator() (ScenarioEvaluator, mockScreeningExecutor) {
	evaluator := ast_eval.EvaluateAstExpression{
		AstEvaluationEnvironmentFactory: func(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
			env := ast_eval.NewAstEvaluationEnvironment()
			env.AddEvaluator(ast.FUNC_CUSTOM_LIST_ACCESS, customListAstMock{})

			return env
		},
	}

	exec := mockScreeningExecutor{
		Mock: &mock.Mock{},
	}

	return ScenarioEvaluator{
		evaluateAstExpression: evaluator,
		evalScreeningUsecase:  exec,
		nameRecognizer:        exec,
		executorFactory:       executor_factory.NewExecutorFactoryStub(),
	}, exec
}

func TestScreeningSkippedWhenDisabled(t *testing.T) {
	eval, _ := getScreeningEvaluator()

	iteration := models.ScenarioIteration{}

	_, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	assert.NoError(t, err)
}

func TestScreeningSkippedWhenTriggerRuleFalse(t *testing.T) {
	eval, _ := getScreeningEvaluator()

	iteration := models.ScenarioIteration{
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule: &ast.Node{Constant: false},
		}},
	}

	sce, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	assert.NoError(t, err)
	assert.Equal(t, models.ScreeningStatusNoHit, sce[0].Status)
}

func TestScreeningErrorWhenNameQueryNotString(t *testing.T) {
	eval, _ := getScreeningEvaluator()

	iteration := models.ScenarioIteration{
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule: &ast.Node{Constant: true},
			Query:       map[string]ast.Node{"name": {Constant: 12}},
		}},
	}

	sce, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	assert.NoError(t, err)
	assert.Equal(t, models.ScreeningStatusError, sce[0].Status)
}

func TestScreeningCalledWhenNameFilterConstant(t *testing.T) {
	eval, exec := getScreeningEvaluator()

	iteration := models.ScenarioIteration{
		OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule: &ast.Node{Constant: true},
			EntityType:  "Thing",
			Query:       map[string]ast.Node{"name": {Constant: "constant string"}},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.ScreeningConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"constant string"},
				},
			},
		},
	}

	_, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{Scenario: models.Scenario{
			OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		}}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"), expectedQuery)

	assert.NoError(t, err)
}

func TestScreeningWithSpecificEntityType(t *testing.T) {
	eval, exec := getScreeningEvaluator()

	iteration := models.ScenarioIteration{
		OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule: &ast.Node{Constant: true},
			EntityType:  "Person",
			Query: map[string]ast.Node{
				"name":      {Constant: "constant string"},
				"birthDate": {Constant: "thedate"},
			},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.ScreeningConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Person",
				Filters: models.OpenSanctionsFilter{
					"name":      []string{"constant string"},
					"birthDate": []string{"thedate"},
				},
			},
		},
	}

	_, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{Scenario: models.Scenario{
			OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		}}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"), expectedQuery)

	assert.NoError(t, err)
}

func TestScreeningCalledWhenNameFilterConcat(t *testing.T) {
	eval, exec := getScreeningEvaluator()

	iteration := models.ScenarioIteration{
		OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule: &ast.Node{Constant: true},
			EntityType:  "Thing",
			Query: map[string]ast.Node{"name": {
				Function:      ast.FUNC_STRING_CONCAT,
				NamedChildren: map[string]ast.Node{"with_separator": {Constant: true}},
				Children: []ast.Node{
					{Constant: "hello"},
					{Constant: "world"},
				},
			}},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.ScreeningConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"hello world"},
				},
			},
		},
	}

	_, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{Scenario: models.Scenario{
			OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		}}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"), expectedQuery)

	assert.NoError(t, err)
}

func TestScreeningCalledWithNameRecognizedLabel(t *testing.T) {
	names := []httpmodels.HTTPNameRecognitionMatch{
		{Type: "Person", Text: "joe finnigan"},
		{Type: "Company", Text: "ACME Inc."},
	}

	eval, exec := getScreeningEvaluator()
	exec.Mock.On("IsConfigured").Return(true)
	exec.Mock.
		On("PerformNameRecognition", mock.Anything, "dinner with joe finnigan").
		Return(names, nil)

	iteration := models.ScenarioIteration{
		OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule:   &ast.Node{Constant: true},
			EntityType:    "Thing",
			Query:         map[string]ast.Node{"name": {Constant: "dinner with joe finnigan"}},
			Preprocessing: models.ScreeningConfigPreprocessing{UseNer: true},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.ScreeningConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"dinner with joe finnigan"},
				},
			},
			{
				Type: "Person",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"joe finnigan"},
				},
			},
			{
				Type: "Organization",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"ACME Inc."},
				},
			},
		},
		InitialQuery: []models.OpenSanctionsCheckQuery{
			{Type: "Thing", Filters: models.OpenSanctionsFilter{
				"name": []string{"dinner with joe finnigan"},
			}},
		},
	}

	_, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{Scenario: models.Scenario{
			OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		}}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"), expectedQuery)

	assert.NoError(t, err)
}

func TestScreeningCalledWithNameRecognitionDisabled(t *testing.T) {
	eval, exec := getScreeningEvaluator()
	exec.Mock.On("IsConfigured").Return(false)

	iteration := models.ScenarioIteration{
		OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule: &ast.Node{Constant: true},
			EntityType:  "Thing",
			Query:       map[string]ast.Node{"name": {Constant: "bob gross"}},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.ScreeningConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"bob gross"},
				},
			},
		},
	}

	_, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{Scenario: models.Scenario{
			OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		}}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"), expectedQuery)

	assert.NoError(t, err)
}

func TestScreeningCalledWithNumbersPreprocessing(t *testing.T) {
	names := []httpmodels.HTTPNameRecognitionMatch{
		{Type: "Person", Text: "444joe finnigan444"},
	}

	eval, exec := getScreeningEvaluator()
	exec.Mock.On("IsConfigured").Return(true)
	exec.Mock.
		On("PerformNameRecognition", mock.Anything, "din2ner 123 with 4 joe fi4n5n65i8gan").
		Return(names, nil)

	iteration := models.ScenarioIteration{
		OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule:   &ast.Node{Constant: true},
			EntityType:    "Thing",
			Query:         map[string]ast.Node{"name": {Constant: "din2ner 123 with 4 joe fi4n5n65i8gan"}},
			Preprocessing: models.ScreeningConfigPreprocessing{UseNer: true, RemoveNumbers: true},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.ScreeningConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"dinner  with  joe finnigan"},
				},
			},
			{
				Type: "Person",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"joe finnigan"},
				},
			},
		},
		InitialQuery: []models.OpenSanctionsCheckQuery{
			{Type: "Thing", Filters: models.OpenSanctionsFilter{
				"name": []string{"din2ner 123 with 4 joe fi4n5n65i8gan"},
			}},
		},
	}

	_, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{Scenario: models.Scenario{
			OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		}}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"), expectedQuery)

	assert.NoError(t, err)
}

func TestScreeningWithLengthPreprocessing(t *testing.T) {
	eval, exec := getScreeningEvaluator()

	exec.Mock.On("IsConfigured").Return(true)
	exec.Mock.
		On("PerformNameRecognition", mock.Anything, "constant string").
		Return([]httpmodels.HTTPNameRecognitionMatch{}, nil)

	iteration := models.ScenarioIteration{
		OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule:   &ast.Node{Constant: true},
			EntityType:    "Thing",
			Query:         map[string]ast.Node{"name": {Constant: "constant string"}},
			Preprocessing: models.ScreeningConfigPreprocessing{SkipIfUnder: 10, UseNer: true},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.ScreeningConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"constant string"},
				},
			},
		},
	}

	_, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{Scenario: models.Scenario{
			OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		}}, DataAccessor{})

	exec.Mock.AssertCalled(t, "PerformNameRecognition", mock.Anything, "constant string")
	exec.Mock.AssertCalled(t, "Execute", context.TODO(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"), expectedQuery)

	assert.NoError(t, err)

	iteration = models.ScenarioIteration{
		OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule:   &ast.Node{Constant: true},
			EntityType:    "Thing",
			Query:         map[string]ast.Node{"name": {Constant: "constant"}},
			Preprocessing: models.ScreeningConfigPreprocessing{SkipIfUnder: 10},
		}},
	}

	expectedQuery = models.OpenSanctionsQuery{
		Config:  iteration.ScreeningConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{},
	}

	_, err = eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{Scenario: models.Scenario{
			OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		}}, DataAccessor{})

	exec.Mock.AssertNotCalled(t, "Execute", context.TODO(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"), expectedQuery)

	assert.NoError(t, err)
}

func TestScreeningWithPreNerLengthPreprocessing(t *testing.T) {
	eval, exec := getScreeningEvaluator()

	exec.Mock.On("IsConfigured").Return(true)

	iteration := models.ScenarioIteration{
		OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule:   &ast.Node{Constant: true},
			EntityType:    "Thing",
			Query:         map[string]ast.Node{"name": {Constant: "short"}},
			Preprocessing: models.ScreeningConfigPreprocessing{SkipIfUnder: 10, UseNer: true},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.ScreeningConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"short"},
				},
			},
		},
	}

	_, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{Scenario: models.Scenario{
			OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		}}, DataAccessor{})

	exec.Mock.AssertNotCalled(t, "PerformNameRecognition", mock.Anything, mock.Anything)
	exec.Mock.AssertNotCalled(t, "Execute", context.TODO(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"), expectedQuery)

	assert.NoError(t, err)
}

func TestScreeningWithListPreprocessing(t *testing.T) {
	eval, exec := getScreeningEvaluator()

	iteration := models.ScenarioIteration{
		OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule:   &ast.Node{Constant: true},
			EntityType:    "Thing",
			Query:         map[string]ast.Node{"name": {Constant: "This Contains Forbidden Words"}},
			Preprocessing: models.ScreeningConfigPreprocessing{IgnoreListId: "ola"},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.ScreeningConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"Contains Words"},
				},
			},
		},
		InitialQuery: []models.OpenSanctionsCheckQuery{
			{Type: "Thing", Filters: models.OpenSanctionsFilter{
				"name": []string{"This Contains Forbidden Words"},
			}},
		},
	}

	_, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{Scenario: models.Scenario{
			OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		}}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"), expectedQuery)

	assert.NoError(t, err)
}

func TestScreeningWithAllPreprocessing(t *testing.T) {
	names := []httpmodels.HTTPNameRecognitionMatch{
		{Type: "Person", Text: "joe2 bill"},
		{Type: "Person", Text: "short"},
		{Type: "Company", Text: "ACME Forbidden Inc."},
	}

	eval, exec := getScreeningEvaluator()
	exec.Mock.On("IsConfigured").Return(true)
	exec.Mock.
		On("PerformNameRecognition", mock.Anything, "original query").
		Return(names, nil)

	iteration := models.ScenarioIteration{
		OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ScreeningConfigs: []models.ScreeningConfig{
			{
				TriggerRule: &ast.Node{Constant: true},
				EntityType:  "Thing",
				Query:       map[string]ast.Node{"name": {Constant: "original query"}},
				Preprocessing: models.ScreeningConfigPreprocessing{
					UseNer:        true,
					SkipIfUnder:   6,
					RemoveNumbers: true,
					IgnoreListId:  "ola",
				},
			},
			{
				TriggerRule: &ast.Node{Constant: true},
				Query:       map[string]ast.Node{"name": {Constant: "short"}},
				Preprocessing: models.ScreeningConfigPreprocessing{
					UseNer:        true,
					SkipIfUnder:   6,
					RemoveNumbers: true,
					IgnoreListId:  "ola",
				},
			},
		},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.ScreeningConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"original query"},
				},
			},
			{
				Type: "Person",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"joe bill"},
				},
			},
			{
				Type: "Organization",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"ACME Inc."},
				},
			},
		},
		InitialQuery: []models.OpenSanctionsCheckQuery{
			{Type: "Thing", Filters: models.OpenSanctionsFilter{"name": []string{"original query"}}},
		},
	}

	_, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{Scenario: models.Scenario{
			OrganizationId: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		}}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"), expectedQuery)

	assert.NoError(t, err)
}

func TestScreeningWithCounterpartyWhitelist(t *testing.T) {
	orgId := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	counterpartyId := "counterparty-123"

	eval, exec := getScreeningEvaluator()

	whitelistItems := []models.ScreeningWhitelist{
		{Id: "whitelist-entity-1", OrgId: orgId, CounterpartyId: counterpartyId, EntityId: "entity-1"},
		{Id: "whitelist-entity-2", OrgId: orgId, CounterpartyId: counterpartyId, EntityId: "entity-2"},
	}

	exec.Mock.
		On("SearchWhitelist", mock.Anything, mock.Anything, orgId, &counterpartyId, (*string)(nil), (*models.UserId)(nil)).
		Return(whitelistItems, nil)

	iteration := models.ScenarioIteration{
		OrganizationId: orgId,
		ScreeningConfigs: []models.ScreeningConfig{{
			TriggerRule:              &ast.Node{Constant: true},
			EntityType:               "Person",
			Query:                    map[string]ast.Node{"name": {Constant: "John Doe"}},
			CounterpartyIdExpression: &ast.Node{Constant: counterpartyId},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.ScreeningConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Person",
				Filters: models.OpenSanctionsFilter{
					"name": []string{"John Doe"},
				},
			},
		},
		WhitelistedEntityIds: []string{"entity-1", "entity-2"},
	}

	_, err := eval.evaluateScreening(context.TODO(), iteration,
		ScenarioEvaluationParameters{Scenario: models.Scenario{
			OrganizationId: orgId,
		}}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), orgId, expectedQuery)
	exec.Mock.AssertCalled(t, "SearchWhitelist", mock.Anything, mock.Anything, orgId, &counterpartyId, (*string)(nil), (*models.UserId)(nil))

	assert.NoError(t, err)
}
