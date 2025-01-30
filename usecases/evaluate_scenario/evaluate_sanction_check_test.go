package evaluate_scenario

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSanctionCheckExecutor struct {
	*mock.Mock
}

func (m mockSanctionCheckExecutor) Execute(ctx context.Context, orgId string,
	cfg models.SanctionCheckConfig, query models.OpenSanctionsQuery,
) (models.SanctionCheck, error) {
	// We are not mocking returned data here, only that the function was called
	// with the appropriate arguments, so we always expect this to be called.
	m.On("Execute", context.TODO(), orgId, cfg, query)
	m.Called(ctx, orgId, cfg, query)

	return models.SanctionCheck{}, nil
}

func getSanctionCheckEvaluatorAndExecutor() (ast_eval.EvaluateAstExpression, mockSanctionCheckExecutor) {
	evaluator := ast_eval.EvaluateAstExpression{
		AstEvaluationEnvironmentFactory: func(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
			return ast_eval.NewAstEvaluationEnvironment()
		},
	}

	return evaluator, mockSanctionCheckExecutor{
		Mock: &mock.Mock{},
	}
}

func TestSanctionCheckSkippedWhenDisabled(t *testing.T) {
	eval, exec := getSanctionCheckEvaluatorAndExecutor()

	iteration := models.ScenarioIteration{}

	_, performed, err := evaluateSanctionCheck(context.TODO(), eval, exec, iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	assert.False(t, performed)
	assert.NoError(t, err)
}

func TestSanctionCheckSkippedWhenTriggerRuleFalse(t *testing.T) {
	eval, exec := getSanctionCheckEvaluatorAndExecutor()

	iteration := models.ScenarioIteration{
		SanctionCheckConfig: &models.SanctionCheckConfig{
			Enabled:     true,
			TriggerRule: &ast.Node{Constant: false},
		},
	}

	_, performed, err := evaluateSanctionCheck(context.TODO(), eval, exec, iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	assert.False(t, performed)
	assert.NoError(t, err)
}

func TestSanctionCheckErrorWhenNameQueryNotString(t *testing.T) {
	eval, exec := getSanctionCheckEvaluatorAndExecutor()

	iteration := models.ScenarioIteration{
		SanctionCheckConfig: &models.SanctionCheckConfig{
			Enabled:     true,
			TriggerRule: &ast.Node{Constant: true},
			Query: models.SanctionCheckConfigQuery{
				Name: ast.Node{Constant: 12},
			},
		},
	}

	_, performed, err := evaluateSanctionCheck(context.TODO(), eval, exec, iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	assert.True(t, performed)
	assert.Error(t, err)
}

func TestSanctionCheckCalledWhenNameFilterConstant(t *testing.T) {
	eval, exec := getSanctionCheckEvaluatorAndExecutor()

	iteration := models.ScenarioIteration{
		SanctionCheckConfig: &models.SanctionCheckConfig{
			Enabled:     true,
			TriggerRule: &ast.Node{Constant: true},
			Query: models.SanctionCheckConfigQuery{
				Name: ast.Node{Constant: "constant string"},
			},
		},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: *iteration.SanctionCheckConfig,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{"constant string"},
		},
	}

	_, performed, err := evaluateSanctionCheck(context.TODO(), eval, exec, iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "",
		*iteration.SanctionCheckConfig, expectedQuery)

	assert.True(t, performed)
	assert.NoError(t, err)
}

func TestSanctionCheckCalledWhenNameFilterConcat(t *testing.T) {
	eval, exec := getSanctionCheckEvaluatorAndExecutor()

	iteration := models.ScenarioIteration{
		SanctionCheckConfig: &models.SanctionCheckConfig{
			Enabled:     true,
			TriggerRule: &ast.Node{Constant: true},
			Query: models.SanctionCheckConfigQuery{
				Name: ast.Node{
					Function:      ast.FUNC_STRING_CONCAT,
					NamedChildren: map[string]ast.Node{"with_separator": {Constant: true}},
					Children: []ast.Node{
						{Constant: "hello"},
						{Constant: "world"},
					},
				},
			},
		},
	}

	expectedQuery := models.OpenSanctionsQuery{
		Config: *iteration.SanctionCheckConfig,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{"hello world"},
		},
	}

	_, performed, err := evaluateSanctionCheck(context.TODO(), eval, exec, iteration,
		ScenarioEvaluationParameters{}, DataAccessor{})

	exec.Mock.AssertCalled(t, "Execute", context.TODO(), "",
		*iteration.SanctionCheckConfig, expectedQuery)

	assert.True(t, performed)
	assert.NoError(t, err)
}
