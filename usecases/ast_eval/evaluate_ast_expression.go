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

func (evaluator EvaluateAstExpression) EvaluateAstExpression(
	ctx context.Context,
	cache *EvaluationCache,
	ruleAstExpression ast.Node,
	organizationId string,
	payload models.ClientObject,
	dataModel models.DataModel,
) (ast.NodeEvaluation, error) {
	environment := evaluator.AstEvaluationEnvironmentFactory(EvaluationEnvironmentFactoryParams{
		OrganizationId:                organizationId,
		ClientObject:                  payload,
		DataModel:                     dataModel,
		DatabaseAccessReturnFakeValue: false,
	})

	evaluation, ok := EvaluateAst(ctx, cache, environment, ruleAstExpression)
	if !ok {
		return evaluation, errors.Join(evaluation.FlattenErrors()...)
	}

	return evaluation, nil
}
