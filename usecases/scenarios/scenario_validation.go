package scenarios

import (
	"errors"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
	"github.com/checkmarble/marble-backend/utils"
)

func ScenarioValidationToError(validation models.ScenarioValidation) error {
	errs := make([]error, 0)

	toError := func(err models.ScenarioValidationError) error {
		return err.Error
	}

	errs = append(errs, utils.Map(validation.Errors, toError)...)

	errs = append(errs, utils.Map(validation.Trigger.Errors, toError)...)
	errs = append(errs, validation.Trigger.TriggerEvaluation.AllErrors()...)

	errs = append(errs, utils.Map(validation.Rules.Errors, toError)...)
	for _, rule := range validation.Rules.Rules {
		errs = append(errs, utils.Map(rule.Errors, toError)...)
		errs = append(errs, rule.RuleEvaluation.AllErrors()...)
	}

	errs = append(errs, utils.Map(validation.Decision.Errors, toError)...)

	return errors.Join(errs...)
}

type ValidateScenarioIteration interface {
	Validate(si ScenarioAndIteration) models.ScenarioValidation
}

type ValidateScenarioIterationImpl struct {
	DataModelRepository             repositories.DataModelRepository
	AstEvaluationEnvironmentFactory ast_eval.AstEvaluationEnvironmentFactory
}

func (validator *ValidateScenarioIterationImpl) Validate(si ScenarioAndIteration) models.ScenarioValidation {
	iteration := si.Iteration

	result := models.NewScenarioValidation()

	// validate Decision
	if iteration.ScoreReviewThreshold == nil {
		result.Decision.Errors = append(result.Trigger.Errors, models.ScenarioValidationError{
			Error: fmt.Errorf("scenario iteration has no ScoreReviewThreshold: \n%w", models.BadParameterError),
			Code:  models.ScoreReviewThresholdRequired,
		})
	}

	if iteration.ScoreRejectThreshold == nil {
		result.Decision.Errors = append(result.Trigger.Errors, models.ScenarioValidationError{
			Error: fmt.Errorf("scenario iteration has no ScoreRejectThreshold: \n%w", models.BadParameterError),
			Code:  models.ScoreRejectThresholdRequired,
		})
	}

	if iteration.ScoreReviewThreshold != nil && iteration.ScoreRejectThreshold != nil && *iteration.ScoreRejectThreshold < *iteration.ScoreReviewThreshold {
		result.Decision.Errors = append(result.Trigger.Errors, models.ScenarioValidationError{
			Error: fmt.Errorf("scenario iteration has ScoreRejectThreshold < ScoreReviewThreshold: \n%w", models.BadParameterError),
			Code:  models.ScoreRejectReviewThresholdsMissmatch,
		})
	}

	dryRunEnvironment, err := validator.makeDryRunEnvironment(si)
	if err != nil {
		result.Errors = append(result.Errors, *err)
		return result
	}

	// validate trigger
	trigger := iteration.TriggerConditionAstExpression
	if trigger == nil {
		result.Trigger.Errors = append(result.Trigger.Errors, models.ScenarioValidationError{
			Error: fmt.Errorf("scenario iteration has no trigger condition ast expression %w", models.BadParameterError),
			Code:  models.TriggerConditionRequired,
		})
	} else {
		result.Trigger.TriggerEvaluation, _ = ast_eval.EvaluateAst(dryRunEnvironment, *trigger)
	}

	// validate each rule
	for _, rule := range iteration.Rules {
		formula := rule.FormulaAstExpression
		ruleValidation := models.NewRuleValidation()
		if formula == nil {
			ruleValidation.Errors = append(ruleValidation.Errors, models.ScenarioValidationError{
				Error: fmt.Errorf("rule has no formula ast expression %w", models.BadParameterError),
				Code:  models.RuleFormulaRequired,
			})
			result.Rules.Rules[rule.Id] = ruleValidation
		} else {
			ruleValidation.RuleEvaluation, _ = ast_eval.EvaluateAst(dryRunEnvironment, *formula)
			result.Rules.Rules[rule.Id] = ruleValidation
		}
	}
	return result
}

func (validator *ValidateScenarioIterationImpl) makeDryRunEnvironment(si ScenarioAndIteration) (ast_eval.AstEvaluationEnvironment, *models.ScenarioValidationError) {
	organizationId := si.Scenario.OrganizationId

	dataModel, err := validator.DataModelRepository.GetDataModel(organizationId)
	if err != nil {
		return ast_eval.AstEvaluationEnvironment{}, &models.ScenarioValidationError{
			Error: fmt.Errorf("could not get data model: %w", err),
			Code:  models.DataModelNotFound,
		}
	}

	table, ok := dataModel.Tables[models.TableName(si.Scenario.TriggerObjectType)]
	if !ok {
		return ast_eval.AstEvaluationEnvironment{}, &models.ScenarioValidationError{
			Error: fmt.Errorf("table %s not found in data model  %w", si.Scenario.TriggerObjectType, models.NotFoundError),
			Code:  models.TrigerObjectNotFound,
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
