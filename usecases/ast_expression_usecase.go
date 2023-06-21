package usecases

import (
	"errors"
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/usecases/ast_eval"
)

type AstExpressionUsecase struct{}

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

func (usecases *AstExpressionUsecase) Run(expression ast.Node) (any, error) {

	inject := ast_eval.NewEvaluatorInjection()
	return ast_eval.EvaluateAst(inject, expression)

}
