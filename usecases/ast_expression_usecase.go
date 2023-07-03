package usecases

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/ast_eval"
	"marble/marble-backend/usecases/ast_eval/evaluate"
	"marble/marble-backend/usecases/organization"
)

type AstExpressionUsecase struct {
	CustomListRepository       repositories.CustomListRepository
	Credentials                models.Credentials
	OrgTransactionFactory      organization.OrgTransactionFactory
	IngestedDataReadRepository repositories.IngestedDataReadRepository
	DatamodelRepository        repositories.DataModelRepository
}

var ErrExpressionValidation = errors.New("expression validation fail")

func (usecase *AstExpressionUsecase) validateRecursif(node ast.Node, allErrors []error) []error {

	attributes, err := node.Function.Attributes()
	if err != nil {
		allErrors = append(allErrors, errors.Join(ErrExpressionValidation, err))
	}

	if attributes.NumberOfArguments != len(node.Children) {
		allErrors = append(allErrors, fmt.Errorf("invalid number of arguments for function %s %w", node.DebugString(), ErrExpressionValidation))
	}

	// TODO: missing named arguments
	// for _, d := attributes.NamedArguments

	// TODO: spurious named arguments

	for _, child := range node.Children {
		allErrors = usecase.validateRecursif(child, allErrors)
	}

	for _, child := range node.NamedChildren {
		allErrors = usecase.validateRecursif(child, allErrors)
	}

	return allErrors
}

func (usecase *AstExpressionUsecase) Validate(node ast.Node) []error {
	return usecase.validateRecursif(node, nil)
}

func (usecase *AstExpressionUsecase) Run(expression ast.Node, payload models.PayloadReader) (any, error) {
	inject := ast_eval.NewEvaluatorInjection()
	inject.AddEvaluator(ast.FUNC_CUSTOM_LIST_ACCESS, evaluate.NewCustomListValuesAccess(usecase.CustomListRepository, usecase.Credentials))
	inject.AddEvaluator(ast.FUNC_DB_ACCESS, evaluate.NewDatabaseAccess(
		usecase.OrgTransactionFactory, usecase.IngestedDataReadRepository,
		usecase.DatamodelRepository, payload, usecase.Credentials))
	return ast_eval.EvaluateAst(inject, expression)

}
