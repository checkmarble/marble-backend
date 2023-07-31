package scenarios

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
)

type ScenarioAndIteration struct {
	scenario  models.Scenario
	iteration models.ScenarioIteration
}

type ValidateScenarioIteration interface {
	Validate(si ScenarioAndIteration) error
}

type ValidateScenarioIterationImpl struct {
	//
}

func (validator *ValidateScenarioIterationImpl) Validate(si ScenarioAndIteration) error {

	iteration := si.iteration.Body

	if iteration.ScoreReviewThreshold == nil {
		return fmt.Errorf("scenario iteration has no ScoreReviewThreshold: \n%w", models.BadParameterError)
	}

	if iteration.ScoreRejectThreshold == nil {
		return fmt.Errorf("scenario iteration has no ScoreRejectThreshold: \n%w", models.BadParameterError)
	}

	if iteration.TriggerConditionAstExpression == nil {
		return fmt.Errorf("scenario iteration has no trigger condition ast expression%w", models.BadParameterError)
	}

	if err := StaticValidation(*iteration.TriggerConditionAstExpression); err != nil {
		return fmt.Errorf("validation of trigger condition failed: %w", err)
	}

	if len(iteration.Rules) < 1 {
		return fmt.Errorf("scenario iteration has no rules: \n%w", models.BadParameterError)
	}
	for _, rule := range iteration.Rules {
		if rule.FormulaAstExpression == nil {
			return fmt.Errorf("scenario iteration rule has no formula ast expression %w", models.BadParameterError)
		}
		// TODO: DRY-run the ast expression

		if err := StaticValidation(*rule.FormulaAstExpression); err != nil {
			return err
		}
	}

	return nil
}

func StaticValidation(node ast.Node) error {
	return errors.Join(staticValidationRescursif(node, nil)...)
}

var ErrExpressionValidation = errors.New("expression validation fail")

func staticValidationRescursif(node ast.Node, allErrors []error) []error {

	attributes, err := node.Function.Attributes()
	if err != nil {
		allErrors = append(allErrors, errors.Join(ErrExpressionValidation, err))
	}

	if attributes.NumberOfArguments != len(node.Children) {
		allErrors = append(allErrors, fmt.Errorf("invalid number of arguments for node [%s] %w", node.DebugString(), ErrExpressionValidation))
	}

	// TODO: missing named arguments
	// for _, d := attributes.NamedArguments

	// TODO: spurious named arguments

	for _, child := range node.Children {
		allErrors = staticValidationRescursif(child, allErrors)
	}

	for _, child := range node.NamedChildren {
		allErrors = staticValidationRescursif(child, allErrors)
	}

	return allErrors
}
