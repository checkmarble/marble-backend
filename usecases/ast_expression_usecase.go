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
	"marble/marble-backend/usecases/security"
)

type AstExpressionUsecase struct {
	EnforceSecurity            security.EnforceSecurity
	OrganizationIdOfContext    string
	CustomListRepository       repositories.CustomListRepository
	OrgTransactionFactory      organization.OrgTransactionFactory
	IngestedDataReadRepository repositories.IngestedDataReadRepository
	DataModelRepository        repositories.DataModelRepository
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
	inject.AddEvaluator(ast.FUNC_CUSTOM_LIST_ACCESS, evaluate.NewCustomListValuesAccess(usecase.CustomListRepository, usecase.EnforceSecurity))
	inject.AddEvaluator(ast.FUNC_DB_ACCESS, evaluate.NewDatabaseAccess(
		usecase.OrgTransactionFactory, usecase.IngestedDataReadRepository,
		usecase.DataModelRepository, payload, usecase.OrganizationIdOfContext))
	return ast_eval.EvaluateAst(inject, expression)

}

type DataAccessesIdentifier struct {
	Varname string `json:"var_name"`
	Vartype string `json:"var_type"`
}

type BuilderIdentifiers struct {
	DataAccesses []DataAccessesIdentifier `json:"data_accesses_identifiers"`
}

func (usecase *AstExpressionUsecase) Identifiers() (BuilderIdentifiers, error) {

	dataModel, err := usecase.DataModelRepository.GetDataModel(nil, usecase.OrganizationIdOfContext)
	if err != nil {
		return BuilderIdentifiers{}, err
	}

	identifiers := BuilderIdentifiers{}

	for tableName, table := range dataModel.Tables {

		for fieldName, field := range table.Fields {

			identifier := DataAccessesIdentifier{

				Varname: fmt.Sprintf("%s.%s", tableName, fieldName),
				Vartype: field.DataType.String(),
			}

			identifiers.DataAccesses = append(identifiers.DataAccesses, identifier)
		}
	}

	return identifiers, nil
}
