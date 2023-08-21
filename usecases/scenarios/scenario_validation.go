package scenarios

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/ast_eval"
	"marble/marble-backend/usecases/ast_eval/evaluate"
)

func ScenarioValidationToError(validation models.ScenarioValidation) error {
	errs := validation.Errs
	errs = append(errs, validation.TriggerEvaluation.AllErrors()...)
	for _, ruleEvaluation := range validation.RulesEvaluations {
		errs = append(errs, ruleEvaluation.AllErrors()...)
	}
	return errors.Join(errs...)
}

type ValidateScenarioIteration interface {
	Validate(si ScenarioAndIteration) models.ScenarioValidation
}

type ValidateScenarioIterationImpl struct {
	DataModelRepository             repositories.DataModelRepository
	AstEvaluationEnvironmentFactory ast_eval.AstEvaluationEnvironmentFactory
}

func (validator *ValidateScenarioIterationImpl) Validate(si ScenarioAndIteration) (result models.ScenarioValidation) {

	iteration := si.Iteration

	result.Errs = make([]error, 0)

	addError := func(err error) {
		result.Errs = append(result.Errs, err)
	}

	if iteration.ScoreReviewThreshold == nil {
		addError(fmt.Errorf("scenario iteration has no ScoreReviewThreshold: \n%w", models.BadParameterError))
	}

	if iteration.ScoreRejectThreshold == nil {
		addError(fmt.Errorf("scenario iteration has no ScoreRejectThreshold: \n%w", models.BadParameterError))
	}

	if len(iteration.Rules) < 1 {
		addError(fmt.Errorf("scenario iteration has no rules: \n%w", models.BadParameterError))
	}

	dryRunEnvironment, err := validator.makeDryRunEnvironment(si)
	if err != nil {
		addError(err)
	}

	// validate trigger
	trigger := iteration.TriggerConditionAstExpression
	if trigger == nil {
		addError(fmt.Errorf("scenario iteration has no trigger condition ast expression %w", models.BadParameterError))
	} else {
		result.TriggerEvaluation, _ = ast_eval.EvaluateAst(dryRunEnvironment, *trigger)
	}

	// validate each rule
	result.RulesEvaluations = make(map[string]ast.NodeEvaluation)
	for _, rule := range iteration.Rules {

		formula := rule.FormulaAstExpression
		if formula == nil {
			result.RulesEvaluations[rule.Id] = ast.NodeEvaluation{
				Errors: []error{fmt.Errorf("rule has no formula ast expression %w", models.BadParameterError)},
			}
		} else {
			result.RulesEvaluations[rule.Id], _ = ast_eval.EvaluateAst(dryRunEnvironment, *formula)
		}
	}

	return result
}

func (validator *ValidateScenarioIterationImpl) makeDryRunEnvironment(si ScenarioAndIteration) (ast_eval.AstEvaluationEnvironment, error) {

	organizationId := si.Scenario.OrganizationId

	dataModel, err := validator.DataModelRepository.GetDataModel(nil, organizationId)
	if err != nil {
		return ast_eval.AstEvaluationEnvironment{}, err
	}

	table, ok := dataModel.Tables[models.TableName(si.Scenario.TriggerObjectType)]
	if !ok {
		return ast_eval.AstEvaluationEnvironment{}, fmt.Errorf("table %s not found in data model  %w", si.Scenario.TriggerObjectType, models.NotFoundError)
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
