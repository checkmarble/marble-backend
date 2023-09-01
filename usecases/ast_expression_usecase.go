package usecases

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/security"
)

type AstExpressionUsecase struct {
	EnforceSecurity      security.EnforceSecurity
	CustomListRepository repositories.CustomListRepository
	DataModelRepository  repositories.DataModelRepository
	ScenarioRepository   repositories.ScenarioReadRepository
}

func NodeLocation(expression ast.Node, target *ast.Node) (string, error) {
	return "", nil
}

type EditorIdentifiers struct {
	CustomListAccessors []ast.Identifier `json:"custom_list_accessors"`
	PayloadAccessors    []ast.Identifier `json:"payload_accessors"`
	DatabaseAccessors   []ast.Identifier `json:"database_accessors"`
	AggregatorAccessors []ast.Identifier `json:"aggregator_accessors"`
}

type EditorOperators struct {
	OperatorAccessors []ast.FuncAttributes `json:"operator_accessors"`
}

func (usecase *AstExpressionUsecase) getLinkedDatabaseIdentifiers(scenario models.Scenario, dataModel models.DataModel) ([]ast.Identifier, error) {
	dataAccessors := []ast.Identifier{}
	var recursiveDatabaseAccessor func(path []string, links map[models.LinkName]models.LinkToSingle) error

	triggerObjectTable, found := dataModel.Tables[models.TableName(scenario.TriggerObjectType)]
	if !found {
		// unexpected error: must be a valid table
		return nil, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}

	recursiveDatabaseAccessor = func(path []string, links map[models.LinkName]models.LinkToSingle) error {
		var baseAccessorName string
		for _, tableName := range path {
			baseAccessorName += tableName + "."
		}
		for linkName, link := range links {
			table, found := dataModel.Tables[link.LinkedTableName]
			if !found {
				// unexpected error: must be a valid table
				return fmt.Errorf("table %s not found in data model", scenario.TriggerObjectType)
			}

			path = append(path, string(linkName))

			for fieldName := range table.Fields {
				dataAccessors = append(dataAccessors, ast.Identifier{
					Name: baseAccessorName + string(linkName) + "." + string(fieldName),
					// TODO fill this in a better way
					Description: "",
					Node: ast.NewNodeDatabaseAccess(
						scenario.TriggerObjectType,
						string(fieldName),
						path,
					),
				})
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

func (usecase *AstExpressionUsecase) getPayloadIdentifiers(scenario models.Scenario, dataModel models.DataModel) ([]ast.Identifier, error) {
	dataAccessors := []ast.Identifier{}

	triggerObjectTable, found := dataModel.Tables[models.TableName(scenario.TriggerObjectType)]
	if !found {
		// unexpected error: must be a valid table
		return nil, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}
	for fieldName := range triggerObjectTable.Fields {
		dataAccessors = append(dataAccessors, ast.Identifier{
			Name:        string(fieldName),
			Description: "",
			Node: ast.Node{
				Function: ast.FUNC_PAYLOAD,
				Constant: nil,
				Children: []ast.Node{
					ast.NewNodeConstant(fieldName),
				},
			},
		})
	}
	return dataAccessors, nil
}

func (usecase *AstExpressionUsecase) getCustomListIdentifiers(organizationId string) ([]ast.Identifier, error) {
	dataAccessors := []ast.Identifier{}

	customLists, err := usecase.CustomListRepository.AllCustomLists(nil, organizationId)
	if err != nil {
		return nil, err
	}
	for _, customList := range customLists {
		dataAccessors = append(dataAccessors, ast.Identifier{
			Name:        customList.Name,
			Description: customList.Description,
			Node:        ast.NewNodeCustomListAccess(customList.Id),
		})
	}
	return dataAccessors, nil
}

func (usecase *AstExpressionUsecase) getAggregatorIdentifiers() ([]ast.Identifier, error) {
	aggregatorAccessors := []ast.Identifier{}
	aggregatorList := ast.GetAllAggregators()

	for _, aggregator := range aggregatorList {
		aggregatorAccessors = append(aggregatorAccessors, ast.Identifier{
			Name:        string(aggregator),
			Description: "",
			Node:        ast.NewNodeAggregator(aggregator),
		})
	}
	return aggregatorAccessors, nil
}

func (usecase *AstExpressionUsecase) EditorIdentifiers(scenarioId string) (EditorIdentifiers, error) {

	scenario, err := usecase.ScenarioRepository.GetScenarioById(nil, scenarioId)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	if err := usecase.EnforceSecurity.ReadOrganization(scenario.OrganizationId); err != nil {
		return EditorIdentifiers{}, err
	}

	dataModel, err := usecase.DataModelRepository.GetDataModel(nil, scenario.OrganizationId)
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

	customListAccessors, err := usecase.getCustomListIdentifiers(scenario.OrganizationId)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	aggregatorAccessors, err := usecase.getAggregatorIdentifiers()
	if err != nil {
		return EditorIdentifiers{}, err
	}

	return EditorIdentifiers{
		CustomListAccessors: customListAccessors,
		PayloadAccessors:    payloadAccessors,
		DatabaseAccessors:   databaseAccessors,
		AggregatorAccessors: aggregatorAccessors,
	}, nil
}

func (usecase *AstExpressionUsecase) EditorOperators() EditorOperators {
	var operatorAccessors []ast.FuncAttributes
	for _, functionType := range ast.FuncOperators {
		operatorAccessors = append(operatorAccessors, ast.FuncAttributesMap[ast.Function(functionType)])
	}
	return EditorOperators{
		OperatorAccessors: operatorAccessors,
	}
}
