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
) (ast.RootNodeEvaluation, error) {
	environment := evaluator.AstEvaluationEnvironmentFactory(EvaluationEnvironmentFactoryParams{
		OrganizationId:                organizationId,
		ClientObject:                  payload,
		DataModel:                     dataModel,
		DatabaseAccessReturnFakeValue: false,
	})

	evaluation, ok := EvaluateAst(ctx, environment, ruleAstExpression)
	if !ok {
		return ast.RootNodeEvaluation{}, errors.Join(evaluation.AllErrors()...)
	}

	rootEvaluation, err := ast.AdaptRootNodeEvaluation(evaluation)
	if err != nil {
		return ast.RootNodeEvaluation{}, errors.Join(models.ErrRuntimeExpression, err)
	}
	return rootEvaluation, nil
}
