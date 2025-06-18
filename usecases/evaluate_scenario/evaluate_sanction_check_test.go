package evaluate_scenario

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSanctionCheckExecutor struct {
	*mock.Mock
}

func (m mockSanctionCheckExecutor) IsConfigured() bool {
	args := m.Called()

	return args.Bool(0)
}

func (m mockSanctionCheckExecutor) Execute(
	ctx context.Context,
	orgId string,
	query models.OpenSanctionsQuery,
) (models.SanctionCheckWithMatches, error) {
	// We are not mocking returned data here, only that the function was called
	// with the appropriate arguments, so we always expect this to be called.
	m.On("Execute", context.TODO(), orgId, query)
	m.Called(ctx, orgId, query)

	return models.SanctionCheckWithMatches{}, nil
}

func (m mockSanctionCheckExecutor) FilterOutWhitelistedMatches(
	ctx context.Context,
	orgId string,
	sanctionCheck models.SanctionCheckWithMatches,
	counterpartyId string,
) (models.SanctionCheckWithMatches, error) {
	// We are not mocking returned data here, only that the function was called
	// with the appropriate arguments, so we always expect this to be called.
	m.On("FilterOutWhitelistedMatches", context.TODO(), orgId, sanctionCheck, counterpartyId)
	m.Called(ctx, orgId, sanctionCheck, counterpartyId)

	return sanctionCheck, nil
}

func (m mockSanctionCheckExecutor) CountWhitelistsForCounterpartyId(
	ctx context.Context,
	orgId string,
	counterpartyId string,
) (int, error) {
	// We are not mocking returned data here, only that the function was called
	// with the appropriate arguments, so we always expect this to be called.
	m.On("CountWhistelistsForCounterparty", context.TODO(), orgId, counterpartyId)
	m.Called(ctx, orgId, counterpartyId)

	return 0, nil
}

func (m mockSanctionCheckExecutor) PerformNameRecognition(ctx context.Context, label string) ([]httpmodels.HTTPNameRecognitionMatch, error) {
	m.On("PerformNameRecognition", ctx, label)
	args := m.Called(context.TODO(), label)

	return args.Get(0).([]httpmodels.HTTPNameRecognitionMatch), args.Error(1)
}

type customListAstMock struct{}

func (customListAstMock) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	return []string{"this", "forbidden"}, nil
}

func getSanctionCheckEvaluator() (ScenarioEvaluator, mockSanctionCheckExecutor) {
	evaluator := ast_eval.EvaluateAstExpression{
		AstEvaluationEnvironmentFactory: func(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
			env := ast_eval.NewAstEvaluationEnvironment()
			env.AddEvaluator(ast.FUNC_CUSTOM_LIST_ACCESS, customListAstMock{})

			return env
		},
	}

	exec := mockSanctionCheckExecutor{
		Mock: &mock.Mock{},
	}

	return ScenarioEvaluator{
		evaluateAstExpression:    evaluator,
		evalSanctionCheckUsecase: exec,
		nameRecognizer:           exec,
	}, exec
}

func TestSanctionCheckSkippedWhenDisabled(t *testing.T) {
	eval, _ := getSanctionCheckEvaluator()

	iteration := models.ScenarioIteration{}

	_, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	assert.NoError(t, err)
}

func TestSanctionCheckSkippedWhenTriggerRuleFalse(t *testing.T) {
	eval, _ := getSanctionCheckEvaluator()

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule: &ast.Node{Constant: false},
		}},
	}

	_, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	assert.NoError(t, err)
}

func TestSanctionCheckErrorWhenNameQueryNotString(t *testing.T) {
	eval, _ := getSanctionCheckEvaluator()

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule: &ast.Node{Constant: true},
			Query:       map[string]ast.Node{"name": {Constant: 12}},
		}},
	}

	sce, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	assert.NoError(t, err)
	assert.Equal(t, models.SanctionStatusError, sce[0].Status)
}

func TestSanctionCheckCalledWhenNameFilterConstant(t *testing.T) {
	eval, exec := getSanctionCheckEvaluator()

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule: &ast.Node{Constant: true},
			EntityType:  "Thing",
			Query:       map[string]ast.Node{"name": {Constant: "constant string"}},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.SanctionCheckConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{"constant string"},
				},
			},
		},
	}

	_, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.NoError(t, err)
}

func TestSanctionCheckWithSpecificEntityType(t *testing.T) {
	eval, exec := getSanctionCheckEvaluator()

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule: &ast.Node{Constant: true},
			EntityType:  "Person",
			Query: map[string]ast.Node{
				"name":      {Constant: "constant string"},
				"birthDate": {Constant: "thedate"},
			},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.SanctionCheckConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Person",
				Filters: models.OpenSanctionCheckFilter{
					"name":      []string{"constant string"},
					"birthDate": []string{"thedate"},
				},
			},
		},
	}

	_, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.NoError(t, err)
}

func TestSanctionCheckCalledWhenNameFilterConcat(t *testing.T) {
	eval, exec := getSanctionCheckEvaluator()

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
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
		Config: iteration.SanctionCheckConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{"hello world"},
				},
			},
		},
	}

	_, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.NoError(t, err)
}

func TestSanctionCheckCalledWithNameRecognizedLabel(t *testing.T) {
	names := []httpmodels.HTTPNameRecognitionMatch{
		{Type: "Person", Text: "joe finnigan"},
		{Type: "Company", Text: "ACME Inc."},
	}

	eval, exec := getSanctionCheckEvaluator()
	exec.Mock.On("IsConfigured").Return(true)
	exec.Mock.
		On("PerformNameRecognition", mock.Anything, "dinner with joe finnigan").
		Return(names, nil)

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule:   &ast.Node{Constant: true},
			Query:         map[string]ast.Node{"name": {Constant: "dinner with joe finnigan"}},
			Preprocessing: models.SanctionCheckConfigPreprocessing{UseNer: true},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.SanctionCheckConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Person",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{"joe finnigan"},
				},
			},
			{
				Type: "Organization",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{"ACME Inc."},
				},
			},
		},
	}

	_, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.NoError(t, err)
}

func TestSanctionCheckCalledWithNameRecognitionDisabled(t *testing.T) {
	eval, exec := getSanctionCheckEvaluator()
	exec.Mock.On("IsConfigured").Return(false)

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule: &ast.Node{Constant: true},
			EntityType:  "Thing",
			Query:       map[string]ast.Node{"name": {Constant: "bob gross"}},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.SanctionCheckConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{"bob gross"},
				},
			},
		},
	}

	_, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.NoError(t, err)
}

func TestSanctionCheckCalledWithNumbersPreprocessing(t *testing.T) {
	names := []httpmodels.HTTPNameRecognitionMatch{
		{Type: "Person", Text: "444joe finnigan444"},
	}

	eval, exec := getSanctionCheckEvaluator()
	exec.Mock.On("IsConfigured").Return(true)
	exec.Mock.
		On("PerformNameRecognition", mock.Anything, "din2ner 123 with 4 joe fi4n5n65i8gan").
		Return(names, nil)

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule:   &ast.Node{Constant: true},
			Query:         map[string]ast.Node{"name": {Constant: "din2ner 123 with 4 joe fi4n5n65i8gan"}},
			Preprocessing: models.SanctionCheckConfigPreprocessing{UseNer: true, RemoveNumbers: true},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.SanctionCheckConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Person",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{"joe finnigan"},
				},
			},
		},
	}

	_, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.NoError(t, err)
}

func TestSanctionCheckWithLengthPreprocessing(t *testing.T) {
	eval, exec := getSanctionCheckEvaluator()

	exec.Mock.On("IsConfigured").Return(true)
	exec.Mock.
		On("PerformNameRecognition", mock.Anything, "constant string").
		Return([]httpmodels.HTTPNameRecognitionMatch{}, nil)

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule:   &ast.Node{Constant: true},
			EntityType:    "Thing",
			Query:         map[string]ast.Node{"name": {Constant: "constant string"}},
			Preprocessing: models.SanctionCheckConfigPreprocessing{SkipIfUnder: 10, UseNer: true},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.SanctionCheckConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{"constant string"},
				},
			},
		},
	}

	_, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "PerformNameRecognition", mock.Anything, "constant string")
	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.NoError(t, err)

	iteration = models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule:   &ast.Node{Constant: true},
			Query:         map[string]ast.Node{"name": {Constant: "constant"}},
			Preprocessing: models.SanctionCheckConfigPreprocessing{SkipIfUnder: 10},
		}},
	}

	expectedQuery = models.OpenSanctionsQuery{
		Config:  iteration.SanctionCheckConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{},
	}

	_, err = eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertNotCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.NoError(t, err)
}

func TestSanctionCheckWithPreNerLengthPreprocessing(t *testing.T) {
	eval, exec := getSanctionCheckEvaluator()

	exec.Mock.On("IsConfigured").Return(true)

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule:   &ast.Node{Constant: true},
			Query:         map[string]ast.Node{"name": {Constant: "short"}},
			Preprocessing: models.SanctionCheckConfigPreprocessing{SkipIfUnder: 10, UseNer: true},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.SanctionCheckConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{"short"},
				},
			},
		},
	}

	_, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertNotCalled(t, "PerformNameRecognition", mock.Anything, mock.Anything)
	exec.Mock.AssertNotCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.NoError(t, err)
}

func TestSanctionCheckWithListPreprocessing(t *testing.T) {
	eval, exec := getSanctionCheckEvaluator()

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule:   &ast.Node{Constant: true},
			EntityType:    "Thing",
			Query:         map[string]ast.Node{"name": {Constant: "This Contains Forbidden Words"}},
			Preprocessing: models.SanctionCheckConfigPreprocessing{IgnoreListId: "ola"},
		}},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.SanctionCheckConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Thing",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{"Contains Words"},
				},
			},
		},
	}

	_, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.NoError(t, err)
}

func TestSanctionCheckWithAllPreprocessing(t *testing.T) {
	names := []httpmodels.HTTPNameRecognitionMatch{
		{Type: "Person", Text: "joe2 bill"},
		{Type: "Person", Text: "short"},
		{Type: "Company", Text: "ACME Forbidden Inc."},
	}

	eval, exec := getSanctionCheckEvaluator()
	exec.Mock.On("IsConfigured").Return(true)
	exec.Mock.
		On("PerformNameRecognition", mock.Anything, "does not matter").
		Return(names, nil)

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{
			{
				TriggerRule: &ast.Node{Constant: true},
				Query:       map[string]ast.Node{"name": {Constant: "does not matter"}},
				Preprocessing: models.SanctionCheckConfigPreprocessing{
					UseNer:        true,
					SkipIfUnder:   6,
					RemoveNumbers: true,
					IgnoreListId:  "ola",
				},
			},
			{
				TriggerRule: &ast.Node{Constant: true},
				Query:       map[string]ast.Node{"name": {Constant: "short"}},
				Preprocessing: models.SanctionCheckConfigPreprocessing{
					UseNer:        true,
					SkipIfUnder:   6,
					RemoveNumbers: true,
					IgnoreListId:  "ola",
				},
			},
		},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: iteration.SanctionCheckConfigs[0],
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type: "Person",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{"joe bill"},
				},
			},
			{
				Type: "Organization",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{"ACME Inc."},
				},
			},
		},
	}

	_, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.NoError(t, err)
}
