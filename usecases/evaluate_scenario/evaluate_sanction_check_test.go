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

func getSanctionCheckEvaluator() (ScenarioEvaluator, mockSanctionCheckExecutor) {
	evaluator := ast_eval.EvaluateAstExpression{
		AstEvaluationEnvironmentFactory: func(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
			return ast_eval.NewAstEvaluationEnvironment()
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

	_, performed, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	assert.False(t, performed)
	assert.NoError(t, err)
}

func TestSanctionCheckSkippedWhenTriggerRuleFalse(t *testing.T) {
	eval, _ := getSanctionCheckEvaluator()

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule: &ast.Node{Constant: false},
		}},
	}

	_, performed, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	assert.False(t, performed)
	assert.NoError(t, err)
}

func TestSanctionCheckErrorWhenNameQueryNotString(t *testing.T) {
	eval, _ := getSanctionCheckEvaluator()

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule: &ast.Node{Constant: true},
			Query:       &ast.Node{Constant: 12},
		}},
	}

	_, performed, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	assert.True(t, performed)
	assert.Error(t, err)
}

func TestSanctionCheckCalledWhenNameFilterConstant(t *testing.T) {
	eval, exec := getSanctionCheckEvaluator()

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule: &ast.Node{Constant: true},
			Query:       &ast.Node{Constant: "constant string"},
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

	_, performed, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.True(t, performed)
	assert.NoError(t, err)
}

func TestSanctionCheckCalledWhenNameFilterConcat(t *testing.T) {
	eval, exec := getSanctionCheckEvaluator()

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule: &ast.Node{Constant: true},
			Query: &ast.Node{
				Function:      ast.FUNC_STRING_CONCAT,
				NamedChildren: map[string]ast.Node{"with_separator": {Constant: true}},
				Children: []ast.Node{
					{Constant: "hello"},
					{Constant: "world"},
				},
			},
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

	_, performed, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.True(t, performed)
	assert.NoError(t, err)
}

func TestSanctionCheckCalledWithNameRecognizedLabel(t *testing.T) {
	names := []httpmodels.HTTPNameRecognitionMatch{
		{Type: "Person", Text: "joe finnigan"},
		{Type: "Company", Text: "acme inc."},
		{Type: "Person", Text: "bill bob"},
	}

	eval, exec := getSanctionCheckEvaluator()
	exec.Mock.On("IsConfigured").Return(true)
	exec.Mock.
		On("PerformNameRecognition", mock.Anything, "dinner with joe finnigan").
		Return(names, nil)

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule: &ast.Node{Constant: true},
			Query:       &ast.Node{Constant: "bob gross"},
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

	_, performed, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.True(t, performed)
	assert.NoError(t, err)
}

func TestSanctionCheckCalledWithNameRecognitionDisabled(t *testing.T) {
	eval, exec := getSanctionCheckEvaluator()
	exec.Mock.On("IsConfigured").Return(false)

	iteration := models.ScenarioIteration{
		SanctionCheckConfigs: []models.SanctionCheckConfig{{
			TriggerRule: &ast.Node{Constant: true},
			Query:       &ast.Node{Constant: "bob gross"},
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

	_, performed, err := eval.evaluateSanctionCheck(context.TODO(), iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "", expectedQuery)

	assert.True(t, performed)
	assert.NoError(t, err)
}
