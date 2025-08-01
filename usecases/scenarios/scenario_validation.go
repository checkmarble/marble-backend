package scenarios

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
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

	for _, sc := range validation.Screenings {
		errs = append(errs, pure_utils.Map(sc.TriggerRule.Errors, toError)...)
		errs = append(errs, sc.TriggerRule.TriggerEvaluation.FlattenErrors()...)
		errs = append(errs, pure_utils.Map(sc.Query.Errors, toError)...)
		errs = append(errs, sc.Query.RuleEvaluation.FlattenErrors()...)

		for _, fieldError := range sc.QueryFields {
			errs = append(errs, pure_utils.Map(fieldError.Errors, toError)...)
		}

		errs = append(errs, sc.CounterpartyIdExpression.RuleEvaluation.FlattenErrors()...)
	}

	return errors.Join(errs...)
}

type ValidateScenarioIteration interface {
	Validate(ctx context.Context, si models.ScenarioAndIteration) models.ScenarioValidation
}

type ValidateScenarioIterationImpl struct {
	AstValidator AstValidator
}

func (self *ValidateScenarioIterationImpl) Validate(ctx context.Context,
	si models.ScenarioAndIteration,
) models.ScenarioValidation {
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

	dryRunEnvironment, err := self.AstValidator.MakeDryRunEnvironment(ctx, si.Scenario)
	if err != nil {
		result.Errors = append(result.Errors, *err)
		return result
	}

	// validate trigger
	trigger := iteration.TriggerConditionAstExpression
	if trigger != nil {
		result.Trigger.TriggerEvaluation, _ = ast_eval.EvaluateAst(ctx, nil, dryRunEnvironment, *trigger)
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
			ruleValidation.RuleEvaluation, _ = ast_eval.EvaluateAst(ctx, nil, dryRunEnvironment, *formula)
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

	// Validate screening trigger and rule
	result.Screenings = make([]models.ScreeningConfigValidation, len(iteration.ScreeningConfigs))

	for idx, scc := range iteration.ScreeningConfigs {
		scResult := models.NewScreeningValidation()

		if scc.TriggerRule != nil {
			triggerRuleEvaluation, _ := ast_eval.EvaluateAst(ctx, nil, dryRunEnvironment,
				*scc.TriggerRule)
			if _, ok := triggerRuleEvaluation.ReturnValue.(bool); !ok {
				scResult.TriggerRule.Errors = append(
					scResult.TriggerRule.Errors, models.ScenarioValidationError{
						Error: errors.Wrap(models.BadParameterError,
							"screening trigger formula does not return a boolean"),
						Code: models.FormulaMustReturnBoolean,
					})
			}
		}

		queryValidation := models.NewRuleValidation()

		if scc.Query == nil || len(scc.Query) == 0 {
			queryValidation.Errors = append(queryValidation.Errors, models.ScenarioValidationError{
				Error: errors.Wrap(models.BadParameterError,
					"screening does not have a query formula"),
				Code: models.RuleFormulaRequired,
			})
		} else {
			providedFields := 0

			for field, fieldAst := range scc.Query {
				queryNameValidation := models.NewRuleValidation()
				providedFields += 1

				queryNameValidation.RuleEvaluation, _ =
					ast_eval.EvaluateAst(ctx, nil, dryRunEnvironment, fieldAst)

				if _, ok := queryNameValidation.RuleEvaluation.ReturnValue.(string); !ok {
					queryNameValidation.Errors = append(
						queryNameValidation.Errors, models.ScenarioValidationError{
							Error: errors.Wrapf(models.BadParameterError,
								"screening field filter '%s' does not return a string", field),
							Code: models.FormulaMustReturnString,
						})
				}

				scResult.QueryFields[field] = queryNameValidation
			}

			if providedFields == 0 {
				queryValidation.Errors = append(queryValidation.Errors, models.ScenarioValidationError{
					Error: errors.Wrap(models.BadParameterError,
						"at least one field filter must be provided"),
					Code: models.RuleFormulaRequired,
				})
			}
		}

		counterpartyIdValidation := models.NewRuleValidation()

		if scc.CounterpartyIdExpression != nil {
			counterpartyIdValidation.RuleEvaluation, _ = ast_eval.EvaluateAst(ctx, nil, dryRunEnvironment,
				*scc.CounterpartyIdExpression)

			if _, ok := counterpartyIdValidation.RuleEvaluation.ReturnValue.(string); !ok {
				counterpartyIdValidation.Errors = append(
					counterpartyIdValidation.Errors, models.ScenarioValidationError{
						Error: errors.Wrap(models.BadParameterError,
							"screening counterparty ID expression does not return a string"),
						Code: models.FormulaMustReturnString,
					})
			}
		}

		scResult.Query = queryValidation
		scResult.CounterpartyIdExpression = counterpartyIdValidation

		result.Screenings[idx] = scResult
	}

	return result
}

type ValidateScenarioAst interface {
	Validate(ctx context.Context, scenario models.Scenario, astNode *ast.Node,
		expectedReturnType ...string) models.AstValidation
}

type ValidateScenarioAstImpl struct {
	AstValidator AstValidator
}

func (self *ValidateScenarioAstImpl) Validate(ctx context.Context,
	scenario models.Scenario,
	astNode *ast.Node,
	expectedReturnTypeStr ...string,
) models.AstValidation {
	result := models.NewAstValidation()

	dryRunEnvironment, err := self.AstValidator.MakeDryRunEnvironment(ctx, scenario)
	if err != nil {
		result.Errors = append(result.Errors, *err)
		return result
	}

	result.Evaluation, _ = ast_eval.EvaluateAst(ctx, nil, dryRunEnvironment, *astNode)

	if len(expectedReturnTypeStr) == 1 {
		expectedReturnType, ok := getTypeFromString(expectedReturnTypeStr[0])
		if !ok {
			result.Errors = append(result.Errors, models.ScenarioValidationError{
				Error: errors.Wrapf(models.BadParameterError,
					"unknown specified type '%s'", expectedReturnTypeStr),
			})
			return result
		}

		astEvaluationReturnType := reflect.TypeOf(result.Evaluation.ReturnValue)
		if astEvaluationReturnType != expectedReturnType {
			result.Errors = append(result.Errors, models.ScenarioValidationError{
				Error: errors.Wrapf(models.BadParameterError,
					"ast node does not return a %s", expectedReturnTypeStr),
				Code: models.FormulaIncorrectReturnType,
			})
			return result
		}
	}

	return result
}

func getTypeFromString(typeStr string) (reflect.Type, bool) {
	switch typeStr {
	case "string":
		return reflect.TypeOf(""), true
	case "int":
		return reflect.TypeOf(int64(0)), true
	case "float":
		return reflect.TypeOf(float64(0)), true
	case "bool":
		return reflect.TypeOf(false), true
	case "datetime":
		return reflect.TypeOf(time.Now()), true
	default:
		return nil, false
	}
}

func hasScoreThresholds(iteration models.ScenarioIteration) bool {
	return iteration.ScoreReviewThreshold != nil &&
		iteration.ScoreBlockAndReviewThreshold != nil &&
		iteration.ScoreDeclineThreshold != nil
}

type AstValidator interface {
	MakeDryRunEnvironment(ctx context.Context, scenario models.Scenario) (
		ast_eval.AstEvaluationEnvironment, *models.ScenarioValidationError)
}

type AstValidatorImpl struct {
	DataModelRepository             repositories.DataModelRepository
	AstEvaluationEnvironmentFactory ast_eval.AstEvaluationEnvironmentFactory
	ExecutorFactory                 executor_factory.ExecutorFactory
}

func (validator *AstValidatorImpl) MakeDryRunEnvironment(ctx context.Context,
	scenario models.Scenario,
) (ast_eval.AstEvaluationEnvironment, *models.ScenarioValidationError) {
	organizationId := scenario.OrganizationId

	dataModel, err := validator.DataModelRepository.GetDataModel(ctx,
		validator.ExecutorFactory.NewExecutor(), organizationId, false)
	if err != nil {
		return ast_eval.AstEvaluationEnvironment{}, &models.ScenarioValidationError{
			Error: errors.Wrap(err, "could not get data model for dry run"),
			Code:  models.DataModelNotFound,
		}
	}

	table, ok := dataModel.Tables[scenario.TriggerObjectType]
	if !ok {
		return ast_eval.AstEvaluationEnvironment{}, &models.ScenarioValidationError{
			Error: errors.Wrap(models.NotFoundError,
				fmt.Sprintf("table %s not found in data model for dry run", scenario.TriggerObjectType)),
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
	}).WithoutOptimizations()

	return env, nil
}
