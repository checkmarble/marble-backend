package scenarios

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

func ScenarioValidationToError(validation models.ScenarioValidation) error {
	errs := make([]error, 0)

	toError := func(err models.ScenarioValidationError) error {
		return err.Error
	}

	errs = append(errs, pure_utils.Map(validation.Errors, toError)...)

	errs = append(errs, pure_utils.Map(validation.Trigger.Errors, toError)...)
	errs = append(errs, validation.Trigger.TriggerEvaluation.FlattenErrors()...)

	errs = append(errs, pure_utils.Map(validation.Rules.Errors, toError)...)
	for _, rule := range validation.Rules.Rules {
		errs = append(errs, pure_utils.Map(rule.Errors, toError)...)
		errs = append(errs, rule.RuleEvaluation.FlattenErrors()...)
	}

	errs = append(errs, pure_utils.Map(validation.Decision.Errors, toError)...)

	return errors.Join(errs...)
}

type ValidateScenarioIteration interface {
	Validate(ctx context.Context, si models.ScenarioAndIteration) models.ScenarioValidation
}

type ValidateScenarioIterationImpl struct {
	DataModelRepository             repositories.DataModelRepository
	AstEvaluationEnvironmentFactory ast_eval.AstEvaluationEnvironmentFactory
	ExecutorFactory                 executor_factory.ExecutorFactory
}

func (validator *ValidateScenarioIterationImpl) Validate(ctx context.Context, si models.ScenarioAndIteration) models.ScenarioValidation {
	iteration := si.Iteration

	result := models.NewScenarioValidation()

	// validate Decision
	if !hasScoreThresholds(iteration) {
		result.Decision.Errors = append(result.Trigger.Errors, models.ScenarioValidationError{
			Error: errors.Wrap(models.BadParameterError,
				"At least one of the 3 score thresholds is missing on the iteration"),
			Code: models.ScoreThresholdMissing,
		})
	}

	if hasScoreThresholds(iteration) &&
		(*iteration.ScoreBlockAndReviewThreshold < *iteration.ScoreReviewThreshold ||
			*iteration.ScoreDeclineThreshold < *iteration.ScoreBlockAndReviewThreshold) {
		result.Decision.Errors = append(result.Trigger.Errors, models.ScenarioValidationError{
			Error: errors.Wrap(models.BadParameterError,
				"scenario iteration thresholds are incorrectly ordered, must be ScoreReviewThreshold<=ScoreBlockAndReviewThreshold<=ScoreDeclineThreshold"),
			Code: models.ScoreThresholdsMismatch,
		})
	}

	dryRunEnvironment, err := validator.makeDryRunEnvironment(ctx, si)
	if err != nil {
		result.Errors = append(result.Errors, *err)
		return result
	}

	// validate trigger
	trigger := iteration.TriggerConditionAstExpression
	if trigger == nil {
		result.Trigger.Errors = append(result.Trigger.Errors, models.ScenarioValidationError{
			Error: errors.Wrap(models.BadParameterError,
				"scenario iteration has no trigger condition ast expression"),
			Code: models.TriggerConditionRequired,
		})
	} else {
		result.Trigger.TriggerEvaluation, _ = ast_eval.EvaluateAst(ctx, dryRunEnvironment, *trigger)
		if _, ok := result.Trigger.TriggerEvaluation.ReturnValue.(bool); !ok {
			result.Trigger.Errors = append(result.Trigger.Errors, models.ScenarioValidationError{
				Error: errors.Wrap(models.BadParameterError,
					"scenario iteration trigger condition does not return a boolean"),
				Code: models.FormulaMustReturnBoolean,
			})
		}
	}

	// validate each rule
	for _, rule := range iteration.Rules {
		formula := rule.FormulaAstExpression
		ruleValidation := models.NewRuleValidation()
		if formula == nil {
			ruleValidation.Errors = append(ruleValidation.Errors, models.ScenarioValidationError{
				Error: errors.Wrap(models.BadParameterError, "rule has no formula ast expression"),
				Code:  models.RuleFormulaRequired,
			})
			result.Rules.Rules[rule.Id] = ruleValidation
		} else {
			ruleValidation.RuleEvaluation, _ = ast_eval.EvaluateAst(ctx, dryRunEnvironment, *formula)
			if _, ok := ruleValidation.RuleEvaluation.ReturnValue.(bool); !ok {
				ruleValidation.Errors = append(ruleValidation.Errors, models.ScenarioValidationError{
					Error: errors.Wrap(models.BadParameterError,
						"rule formula does not return a boolean"),
					Code: models.FormulaMustReturnBoolean,
				})
			}
			result.Rules.Rules[rule.Id] = ruleValidation
		}
	}
	return result
}

func hasScoreThresholds(iteration models.ScenarioIteration) bool {
	return iteration.ScoreReviewThreshold != nil &&
		iteration.ScoreBlockAndReviewThreshold != nil &&
		iteration.ScoreDeclineThreshold != nil
}

func (validator *ValidateScenarioIterationImpl) makeDryRunEnvironment(ctx context.Context,
	si models.ScenarioAndIteration,
) (ast_eval.AstEvaluationEnvironment, *models.ScenarioValidationError) {
	organizationId := si.Scenario.OrganizationId

	dataModel, err := validator.DataModelRepository.GetDataModel(ctx,
		validator.ExecutorFactory.NewExecutor(), organizationId, false)
	if err != nil {
		return ast_eval.AstEvaluationEnvironment{}, &models.ScenarioValidationError{
			Error: errors.Wrap(err, "could not get data model for dry run"),
			Code:  models.DataModelNotFound,
		}
	}

	table, ok := dataModel.Tables[si.Scenario.TriggerObjectType]
	if !ok {
		return ast_eval.AstEvaluationEnvironment{}, &models.ScenarioValidationError{
			Error: errors.Wrap(models.NotFoundError,
				fmt.Sprintf("table %s not found in data model for dry run", si.Scenario.TriggerObjectType)),
			Code: models.TrigerObjectNotFound,
		}
	}

	clientObject := models.ClientObject{
		TableName: table.Name,
		Data:      evaluate.DryRunPayload(table),
	}

	env := validator.AstEvaluationEnvironmentFactory(ast_eval.EvaluationEnvironmentFactoryParams{
		OrganizationId:                organizationId,
		ClientObject:                  clientObject,
		DataModel:                     dataModel,
		DatabaseAccessReturnFakeValue: true,
	})
	return env, nil
}
