package usecases

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/ast_eval"
	"marble/marble-backend/usecases/organization"
	"marble/marble-backend/usecases/security"
)

type AstExpressionUsecase struct {
	EnforceSecurity                 security.EnforceSecurity
	OrganizationIdOfContext         string
	CustomListRepository            repositories.CustomListRepository
	OrgTransactionFactory           organization.OrgTransactionFactory
	IngestedDataReadRepository      repositories.IngestedDataReadRepository
	DataModelRepository             repositories.DataModelRepository
	ScenarioRepository              repositories.ScenarioReadRepository
	ScenarioIterationReadRepository repositories.ScenarioIterationReadRepository
	EvaluatorInjectionFactory       func(organizationId string, payload models.PayloadReader) ast_eval.EvaluatorInjection
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

	environment := usecase.EvaluatorInjectionFactory(usecase.OrganizationIdOfContext, payload)
	return ast_eval.EvaluateAst(environment, expression)
}

type EditorIdentifiers struct {
	DataAccessors []ast.Node `json:"data_accessors"`
}

func (usecase *AstExpressionUsecase) EditorIdentifiers(scenarioId string) (EditorIdentifiers, error) {

	scenario, err := usecase.ScenarioRepository.GetScenarioById(nil, scenarioId)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	if err := usecase.EnforceSecurity.ReadOrganization(scenario.OrganizationID); err != nil {
		return EditorIdentifiers{}, err
	}

	dataModel, err := usecase.DataModelRepository.GetDataModel(nil, scenario.OrganizationID)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	triggerObjectTable, found := dataModel.Tables[models.TableName(scenario.TriggerObjectType)]
	if !found {
		// unexpected error: must be a valid table
		return EditorIdentifiers{}, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}

	var dataAccessors []ast.Node

	var recursiveDatabaseAccessor func(path []string, links map[models.LinkName]models.LinkToSingle) error

	recursiveDatabaseAccessor = func(path []string, links map[models.LinkName]models.LinkToSingle) error {
		for linkName, link := range links {

			table, found := dataModel.Tables[link.LinkedTableName]
			if !found {
				// unexpected error: must be a valid table
				return fmt.Errorf("table %s not found in data model", scenario.TriggerObjectType)
			}

			path = append(path, string(linkName))

			for fieldName := range table.Fields {
				dataAccessors = append(dataAccessors, ast.NewNodeDatabaseAccess(
					scenario.TriggerObjectType,
					string(fieldName),
					path,
				))
			}

			if err := recursiveDatabaseAccessor(path, table.LinksToSingle); err != nil {
				return err
			}
		}
		return nil
	}

	var path []string
	if err := recursiveDatabaseAccessor(path, triggerObjectTable.LinksToSingle); err != nil {
		return EditorIdentifiers{}, err
	}

	return EditorIdentifiers{
		DataAccessors: dataAccessors,
	}, nil
}

func (usecase *AstExpressionUsecase) SaveIterationWithAstExpression(expression ast.Node, scenarioIterationId string) error {

	// TODO: use refactored repo that do not request context.Background and OrganizationIdOfContext
	scenarioIteration, err := usecase.ScenarioIterationReadRepository.GetScenarioIteration(context.Background(), usecase.OrganizationIdOfContext, scenarioIterationId)
	if err != nil {
		return err
	}

	// fetch scenario and enforce security
	scenario, err := usecase.ScenarioRepository.GetScenarioById(nil, scenarioIteration.ScenarioID)
	if err != nil {
		return err
	}

	if err := usecase.EnforceSecurity.ReadOrganization(scenario.OrganizationID); err != nil {
		return err
	}

	// TODO: write scenarioIteration.Body.Rules to `scenarioIteration`

	return nil
}
