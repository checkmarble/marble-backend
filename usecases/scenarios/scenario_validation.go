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
	errs = append(errs, validation.Trigger.TriggerEvaluation.AllErrors()...)

	errs = append(errs, pure_utils.Map(validation.Rules.Errors, toError)...)
	for _, rule := range validation.Rules.Rules {
		errs = append(errs, pure_utils.Map(rule.Errors, toError)...)
		errs = append(errs, rule.RuleEvaluation.AllErrors()...)
	}

	errs = append(errs, pure_utils.Map(validation.Decision.Errors, toError)...)

	return errors.Join(errs...)
}

type ValidateScenarioIteration interface {
	Validate(ctx context.Context, si ScenarioAndIteration) models.ScenarioValidation
}

type ValidateScenarioIterationImpl struct {
	DataModelRepository             repositories.DataModelRepository
	AstEvaluationEnvironmentFactory ast_eval.AstEvaluationEnvironmentFactory
	ExecutorFactory                 executor_factory.ExecutorFactory
}

func (validator *ValidateScenarioIterationImpl) Validate(ctx context.Context, si ScenarioAndIteration) models.ScenarioValidation {
	iteration := si.Iteration

	result := models.NewScenarioValidation()

	// validate Decision
	if iteration.ScoreReviewThreshold == nil {
		result.Decision.Errors = append(result.Trigger.Errors, models.ScenarioValidationError{
			Error: errors.Wrap(models.BadParameterError,
				"scenario iteration has no ScoreReviewThreshold"),
			Code: models.ScoreReviewThresholdRequired,
		})
	}

	if iteration.ScoreRejectThreshold == nil {
		result.Decision.Errors = append(result.Trigger.Errors, models.ScenarioValidationError{
			Error: errors.Wrap(models.BadParameterError,
				"scenario iteration has no ScoreRejectThreshold"),
			Code: models.ScoreRejectThresholdRequired,
		})
	}

	if iteration.ScoreReviewThreshold != nil && iteration.ScoreRejectThreshold != nil &&
		*iteration.ScoreRejectThreshold < *iteration.ScoreReviewThreshold {
		result.Decision.Errors = append(result.Trigger.Errors, models.ScenarioValidationError{
			Error: errors.Wrap(models.BadParameterError,
				"scenario iteration has ScoreRejectThreshold < ScoreReviewThreshold"),
			Code: models.ScoreRejectReviewThresholdsMissmatch,
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
			result.Rules.Rules[rule.Id] = ruleValidation
		}
	}
	return result
}

func (validator *ValidateScenarioIterationImpl) makeDryRunEnvironment(ctx context.Context,
	si ScenarioAndIteration,
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

	table, ok := dataModel.Tables[models.TableName(si.Scenario.TriggerObjectType)]
	if !ok {
		return ast_eval.AstEvaluationEnvironment{}, &models.ScenarioValidationError{
			Error: errors.Wrap(models.NotFoundError,
				fmt.Sprintf("table %s not found in data model for dry run", si.Scenario.TriggerObjectType)),
			Code: models.TrigerObjectNotFound,
		}
	}

	payload := models.ClientObject{
		TableName: table.Name,
		Data:      evaluate.DryRunPayload(table),
	}

	env := validator.AstEvaluationEnvironmentFactory(ast_eval.EvaluationEnvironmentFactoryParams{
		OrganizationId:                organizationId,
		Payload:                       payload,
		DataModel:                     dataModel,
		DatabaseAccessReturnFakeValue: true,
	})
	return env, nil
}
