package ast_eval

import (
	"errors"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

type EvaluateRuleAstExpression struct {
	AstEvaluationEnvironmentFactory AstEvaluationEnvironmentFactory
}

func (evaluator *EvaluateRuleAstExpression) EvaluateRuleAstExpression(ruleAstExpression ast.Node, organizationId string, payload models.PayloadReader, dataModel models.DataModel) (bool, error) {

	environment := evaluator.AstEvaluationEnvironmentFactory(EvaluationEnvironmentFactoryParams{
		OrganizationId:                organizationId,
		Payload:                       payload,
		DataModel:                     dataModel,
		DatabaseAccessReturnFakeValue: false,
	})

	evaluation, ok := EvaluateAst(environment, ruleAstExpression)

	if !ok {
		return false, errors.Join(evaluation.AllErrors()...)
	}
	result := evaluation.ReturnValue

	if value, ok := result.(bool); ok {
		return value, nil
	}

	return false, fmt.Errorf("rule ast expression does not return a boolean, '%v' instead %w", result, models.ErrRuntimeExpression)
}
