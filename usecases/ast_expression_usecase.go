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
	ScenarioRepository         repositories.ScenarioReadRepository
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
	inject.AddEvaluator(ast.FUNC_PAYLOAD, evaluate.NewPayload(ast.FUNC_PAYLOAD, payload))
	return ast_eval.EvaluateAst(inject, expression)

}

type EditorIdentifiers struct {
	CustomListAccessors []ast.Node `json:"custom_list_accessors"`
	PayloadAccessors    []ast.Node `json:"payload_accessors"`
	DatabaseAccessors   []ast.Node `json:"database_accessors"`
}

func (usecase *AstExpressionUsecase) getLinkedDatabaseIdentifiers(scenario models.Scenario, dataModel models.DataModel) ([]ast.Node, error) {
	var dataAccessors []ast.Node
	var recursiveDatabaseAccessor func(path []string, links map[models.LinkName]models.LinkToSingle) error

	triggerObjectTable, found := dataModel.Tables[models.TableName(scenario.TriggerObjectType)]
	if !found {
		// unexpected error: must be a valid table
		return nil, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}

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
		return nil, err
	}
	return dataAccessors, nil
}

func (usecase *AstExpressionUsecase) getPayloadIdentifiers(scenario models.Scenario, dataModel models.DataModel) ([]ast.Node, error) {
	var dataAccessors []ast.Node

	triggerObjectTable, found := dataModel.Tables[models.TableName(scenario.TriggerObjectType)]
	if !found {
		// unexpected error: must be a valid table
		return nil, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}
	for fieldName, _ := range triggerObjectTable.Fields {
		dataAccessors = append(dataAccessors, ast.Node{
			Function: ast.FUNC_PAYLOAD,
			Constant: nil,
			Children: []ast.Node{
				ast.NewNodeConstant(fieldName),
			},
		})
	}
	return dataAccessors, nil
}

func (usecase *AstExpressionUsecase) getCustomListIdentifiers(organizationId string) ([]ast.Node, error) {
	var dataAccessors []ast.Node

	customLists, err := usecase.CustomListRepository.AllCustomLists(nil, organizationId)
	if err != nil {
		return nil, err
	}
	for _, customList := range customLists {
		dataAccessors = append(dataAccessors, ast.NewNodeCustomListAccess(customList.Id))
	}
	return dataAccessors, nil
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

	databaseAccessors, err := usecase.getLinkedDatabaseIdentifiers(scenario, dataModel)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	payloadAccessors, err := usecase.getPayloadIdentifiers(scenario, dataModel)
	if err != nil {
		return EditorIdentifiers{}, err
	}
	
	customListAccessors, err := usecase.getCustomListIdentifiers(scenario.OrganizationID)
	if err != nil {
		return EditorIdentifiers{}, err
	}
	
	return EditorIdentifiers{
		CustomListAccessors: customListAccessors,
		PayloadAccessors:    payloadAccessors,
		DatabaseAccessors:   databaseAccessors,
	}, nil
}
