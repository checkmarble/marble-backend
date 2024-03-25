package ast_eval

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

type EvaluateAstExpression struct {
	AstEvaluationEnvironmentFactory AstEvaluationEnvironmentFactory
}

func (evaluator *EvaluateAstExpression) EvaluateAstExpression(
	ctx context.Context,
	ruleAstExpression ast.Node,
	organizationId string,
	payload models.ClientObject,
	dataModel models.DataModel,
) (bool, ast.NodeEvaluation, error) {
	environment := evaluator.AstEvaluationEnvironmentFactory(EvaluationEnvironmentFactoryParams{
		OrganizationId:                organizationId,
		ClientObject:                  payload,
		DataModel:                     dataModel,
		DatabaseAccessReturnFakeValue: false,
	})

	evaluation, ok := EvaluateAst(ctx, environment, ruleAstExpression)
	if !ok {
		return false, evaluation, errors.Join(evaluation.FlattenErrors()...)
	}

	returnValue, err := evaluation.GetBoolReturnValue()
	if err != nil {
		return false, evaluation, errors.Join(ast.ErrRuntimeExpression, err)
	}
	return returnValue, evaluation, nil
}
