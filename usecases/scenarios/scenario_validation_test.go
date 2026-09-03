package scenarios

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/utils"
)

func TestValidateScenarioIterationImpl_Validate(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	scenario := models.Scenario{
		Id:                pure_utils.NewId().String(),
		OrganizationId:    pure_utils.NewId(),
		Name:              "scenario_name",
		Description:       "description",
		TriggerObjectType: "object_type",
		CreatedAt:         time.Now(),
		LiveVersionID:     utils.Ptr(pure_utils.NewId().String()),
	}

	scenarioIterationID := pure_utils.NewId().String()
	scenarioIteration := models.ScenarioIteration{
		Id:             scenarioIterationID,
		OrganizationId: scenario.OrganizationId,
		ScenarioId:     scenario.Id,
		Version:        utils.Ptr(1),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		TriggerConditionAstExpression: utils.Ptr(ast.Node{
			Constant: true,
		}),
		Rules: []models.Rule{
			{
				Id:                  "rule",
				ScenarioIterationId: scenarioIterationID,
				OrganizationId:      scenario.OrganizationId,
				DisplayOrder:        0,
				Name:                "rule",
				Description:         "description",
				FormulaAstExpression: utils.Ptr(ast.Node{
					Function: ast.FUNC_GREATER,
					Constant: nil,
					Children: []ast.Node{
						{
							Constant: 10,
						},
						{
							Constant: 100,
						},
					},
				}),
				ScoreModifier: 10,
				CreatedAt:     time.Now(),
			},
		},
		ScoreReviewThreshold:         utils.Ptr(100),
		ScoreBlockAndReviewThreshold: utils.Ptr(1000),
		ScoreDeclineThreshold:        utils.Ptr(1000),
		Schedule:                     "schedule",
	}

	exec := new(mocks.Executor)
	executorFactory := new(mocks.ExecutorFactory)
	executorFactory.On("NewExecutor").Once().Return(exec)
	mdmr := new(mocks.DataModelRepository)
	mdmr.On("GetDataModel", ctx, exec, scenario.OrganizationId, false, mock.Anything).
		Return(models.DataModel{
			Version: "version",
			Tables: map[string]models.Table{
				"object_type": {
					Name: "object_type",
					Fields: map[string]models.Field{
						"id": {
							DataType: models.Int,
						},
					},
					LinksToSingle: nil,
				},
			},
		}, nil)

	validator := AstValidatorImpl{
		DataModelRepository: mdmr,
		AstEvaluationEnvironmentFactory: func(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
			return ast_eval.NewAstEvaluationEnvironment().WithoutOptimizations()
		},
		ExecutorFactory: executorFactory,
	}

	siValidator := ValidateScenarioIterationImpl{
		AstValidator: &validator,
	}

	result := siValidator.Validate(ctx, models.ScenarioAndIteration{
		Scenario:  scenario,
		Iteration: scenarioIteration,
	})
	assert.Empty(t, ScenarioValidationToError(result))
}

func TestValidateScenarioIterationImpl_Validate_notBool(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	scenario := models.Scenario{
		Id:                pure_utils.NewId().String(),
		OrganizationId:    pure_utils.NewId(),
		Name:              "scenario_name",
		Description:       "description",
		TriggerObjectType: "object_type",
		CreatedAt:         time.Now(),
		LiveVersionID:     utils.Ptr(pure_utils.NewId().String()),
	}

	scenarioIterationID := pure_utils.NewId().String()
	scenarioIteration := models.ScenarioIteration{
		Id:             scenarioIterationID,
		OrganizationId: scenario.OrganizationId,
		ScenarioId:     scenario.Id,
		Version:        utils.Ptr(1),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		TriggerConditionAstExpression: utils.Ptr(ast.Node{
			Constant: 100, // should be a boolean, resulting in an error
		}),
		Rules: []models.Rule{
			{
				Id:                  "rule",
				ScenarioIterationId: scenarioIterationID,
				OrganizationId:      scenario.OrganizationId,
				DisplayOrder:        0,
				Name:                "rule",
				Description:         "description",
				FormulaAstExpression: utils.Ptr(ast.Node{
					Function: ast.FUNC_GREATER,
					Constant: nil,
					Children: []ast.Node{
						{
							Constant: 10,
						},
						{
							Constant: 100,
						},
					},
				}),
				ScoreModifier: 10,
				CreatedAt:     time.Now(),
			},
		},
		ScoreReviewThreshold:         utils.Ptr(100),
		ScoreBlockAndReviewThreshold: utils.Ptr(1000),
		ScoreDeclineThreshold:        utils.Ptr(1000),
		Schedule:                     "schedule",
	}

	exec := new(mocks.Executor)
	executorFactory := new(mocks.ExecutorFactory)
	executorFactory.On("NewExecutor").Once().Return(exec)
	mdmr := new(mocks.DataModelRepository)
	mdmr.On("GetDataModel", ctx, exec, scenario.OrganizationId, false, mock.Anything).
		Return(models.DataModel{
			Version: "version",
			Tables: map[string]models.Table{
				"object_type": {
					Name: "object_type",
					Fields: map[string]models.Field{
						"id": {
							DataType: models.Int,
						},
					},
					LinksToSingle: nil,
				},
			},
		}, nil)

	validator := AstValidatorImpl{
		DataModelRepository: mdmr,
		AstEvaluationEnvironmentFactory: func(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
			return ast_eval.NewAstEvaluationEnvironment().WithoutOptimizations()
		},
		ExecutorFactory: executorFactory,
	}

	siValidator := ValidateScenarioIterationImpl{
		AstValidator: &validator,
	}

	result := siValidator.Validate(ctx, models.ScenarioAndIteration{
		Scenario:  scenario,
		Iteration: scenarioIteration,
	})
	assert.NotEmpty(t, ScenarioValidationToError(result))
}

func TestValidationShouldBypassCircuitBreaking(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	scenario := models.Scenario{
		Id:                pure_utils.NewId().String(),
		OrganizationId:    pure_utils.NewId(),
		Name:              "scenario_name",
		Description:       "description",
		TriggerObjectType: "object_type",
		CreatedAt:         time.Now(),
		LiveVersionID:     utils.Ptr(pure_utils.NewId().String()),
	}

	scenarioIterationID := pure_utils.NewId().String()
	scenarioIteration := models.ScenarioIteration{
		Id:             scenarioIterationID,
		OrganizationId: scenario.OrganizationId,
		ScenarioId:     scenario.Id,
		Version:        utils.Ptr(1),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		TriggerConditionAstExpression: utils.Ptr(ast.Node{
			Constant: true,
		}),
		Rules: []models.Rule{
			{
				Id:                  "rule",
				ScenarioIterationId: scenarioIterationID,
				OrganizationId:      scenario.OrganizationId,
				DisplayOrder:        0,
				Name:                "rule",
				Description:         "description",
				FormulaAstExpression: utils.Ptr(ast.Node{
					Function: ast.FUNC_AND,
					Constant: nil,
					Children: []ast.Node{
						{
							Function: ast.FUNC_EQUAL,
							Children: []ast.Node{
								{Constant: 100},
								{Constant: 101},
							},
						},
						{
							Function: ast.FUNC_EQUAL,
							Children: []ast.Node{
								{Constant: 100},
								{Constant: "oplop"},
							},
						},
					},
				}),
				ScoreModifier: 10,
				CreatedAt:     time.Now(),
			},
		},
		ScoreReviewThreshold:         utils.Ptr(100),
		ScoreBlockAndReviewThreshold: utils.Ptr(1000),
		ScoreDeclineThreshold:        utils.Ptr(1000),
		Schedule:                     "schedule",
	}

	exec := new(mocks.Executor)
	executorFactory := new(mocks.ExecutorFactory)
	executorFactory.On("NewExecutor").Once().Return(exec)
	mdmr := new(mocks.DataModelRepository)
	mdmr.On("GetDataModel", ctx, exec, scenario.OrganizationId, false, mock.Anything).
		Return(models.DataModel{
			Version: "version",
			Tables: map[string]models.Table{
				"object_type": {
					Name: "object_type",
					Fields: map[string]models.Field{
						"id": {
							DataType: models.Int,
						},
					},
					LinksToSingle: nil,
				},
			},
		}, nil)

	validator := AstValidatorImpl{
		DataModelRepository: mdmr,
		AstEvaluationEnvironmentFactory: func(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
			return ast_eval.NewAstEvaluationEnvironment().WithoutOptimizations()
		},
		ExecutorFactory: executorFactory,
	}

	siValidator := ValidateScenarioIterationImpl{
		AstValidator: &validator,
	}

	result := siValidator.Validate(ctx, models.ScenarioAndIteration{
		Scenario:  scenario,
		Iteration: scenarioIteration,
	})
	assert.NotEmpty(t, ScenarioValidationToError(result))
}

func TestValidateScenarioAstNumericSwitch(t *testing.T) {
	validator := ValidateScenarioAstImpl{
		AstValidator: staticAstValidator{
			environment: ast_eval.NewAstEvaluationEnvironment().WithoutOptimizations(),
		},
	}

	t.Run("returns a float that can be used in a boolean comparison", func(t *testing.T) {
		numericSwitch := ast.Node{Function: ast.FUNC_SWITCH}.
			AddChild(ast.Node{Function: ast.FUNC_CASE}.
				AddChild(ast.NewNodeConstant(true)).
				AddChild(ast.NewNodeConstant(20))).
			AddNamedChild("fallback", ast.NewNodeConstant(10))
		switchValidation := validator.Validate(context.Background(), models.Scenario{}, &numericSwitch, "float")
		assert.Empty(t, switchValidation.Errors)
		assert.Equal(t, 20.0, switchValidation.Evaluation.ReturnValue)

		formula := ast.Node{Function: ast.FUNC_GREATER}.
			AddChild(ast.NewNodeConstant(30)).
			AddChild(numericSwitch)

		validation := validator.Validate(context.Background(), models.Scenario{}, &formula, "bool")

		assert.Empty(t, validation.Errors)
		assert.Empty(t, validation.Evaluation.FlattenErrors())
		assert.Equal(t, true, validation.Evaluation.ReturnValue)
		assert.Equal(t, 20.0, validation.Evaluation.Children[1].ReturnValue)
	})

	t.Run("targets a malformed case value", func(t *testing.T) {
		numericSwitch := ast.Node{Function: ast.FUNC_SWITCH}.
			AddChild(ast.Node{Function: ast.FUNC_CASE}.
				AddChild(ast.NewNodeConstant(true)).
				AddChild(ast.NewNodeConstant("not a number"))).
			AddNamedChild("fallback", ast.NewNodeConstant(10))
		formula := ast.Node{Function: ast.FUNC_GREATER}.
			AddChild(ast.NewNodeConstant(30)).
			AddChild(numericSwitch)

		validation := validator.Validate(context.Background(), models.Scenario{}, &formula, "bool")

		caseErrors := validation.Evaluation.Children[1].Children[0].Errors
		assert.NotEmpty(t, caseErrors)
		var argumentErr ast.ArgumentError
		assert.ErrorAs(t, caseErrors[0], &argumentErr)
		assert.Equal(t, ast.NewArgumentError(1), argumentErr)
	})

	t.Run("targets a malformed fallback", func(t *testing.T) {
		numericSwitch := ast.Node{Function: ast.FUNC_SWITCH}.
			AddChild(ast.Node{Function: ast.FUNC_CASE}.
				AddChild(ast.NewNodeConstant(false)).
				AddChild(ast.NewNodeConstant(20))).
			AddNamedChild("fallback", ast.NewNodeConstant("not a number"))
		formula := ast.Node{Function: ast.FUNC_GREATER}.
			AddChild(ast.NewNodeConstant(30)).
			AddChild(numericSwitch)

		validation := validator.Validate(context.Background(), models.Scenario{}, &formula, "bool")

		switchErrors := validation.Evaluation.Children[1].Errors
		assert.NotEmpty(t, switchErrors)
		var argumentErr ast.ArgumentError
		assert.ErrorAs(t, switchErrors[0], &argumentErr)
		assert.Equal(t, ast.NewNamedArgumentError("fallback"), argumentErr)
	})
}

type staticAstValidator struct {
	environment ast_eval.AstEvaluationEnvironment
}

func (v staticAstValidator) MakeDryRunEnvironment(
	context.Context,
	models.Scenario,
) (ast_eval.AstEvaluationEnvironment, *models.ScenarioValidationError) {
	return v.environment, nil
}
