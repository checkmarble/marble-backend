package usecases

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type AstExpressionUsecaseRepository interface {
	GetScenarioById(tx repositories.Transaction, scenarioId string) (models.Scenario, error)
}

type AstExpressionUsecase struct {
	EnforceSecurity     security.EnforceSecurityScenario
	DataModelRepository repositories.DataModelRepository
	Repository          AstExpressionUsecaseRepository
}

func NodeLocation(expression ast.Node, target *ast.Node) (string, error) {
	return "", nil
}

type EditorIdentifiers struct {
	PayloadAccessors  []ast.Node `json:"payload_accessors"`
	DatabaseAccessors []ast.Node `json:"database_accessors"`
}

type EditorOperators struct {
	OperatorAccessors []ast.FuncAttributes `json:"operator_accessors"`
}

func (usecase *AstExpressionUsecase) getLinkedDatabaseIdentifiers(scenario models.Scenario, dataModel models.DataModel) ([]ast.Node, error) {
	dataAccessors := []ast.Node{}
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
				dataAccessors = append(dataAccessors,
					ast.NewNodeDatabaseAccess(
						scenario.TriggerObjectType,
						string(fieldName),
						path,
					),
				)
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
	dataAccessors := []ast.Node{}

	triggerObjectTable, found := dataModel.Tables[models.TableName(scenario.TriggerObjectType)]
	if !found {
		// unexpected error: must be a valid table
		return nil, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}
	for fieldName := range triggerObjectTable.Fields {
		dataAccessors = append(dataAccessors,
			ast.Node{
				Function: ast.FUNC_PAYLOAD,
				Constant: nil,
				Children: []ast.Node{
					ast.NewNodeConstant(fieldName),
				},
			},
		)
	}
	return dataAccessors, nil
}

func (usecase *AstExpressionUsecase) EditorIdentifiers(scenarioId string) (EditorIdentifiers, error) {

	scenario, err := usecase.Repository.GetScenarioById(nil, scenarioId)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	if err := usecase.EnforceSecurity.ReadScenario(scenario); err != nil {
		return EditorIdentifiers{}, err
	}

	dataModel, err := usecase.DataModelRepository.GetDataModel(scenario.OrganizationId)
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

	return EditorIdentifiers{
		PayloadAccessors:  payloadAccessors,
		DatabaseAccessors: databaseAccessors,
	}, nil
}

func (usecase *AstExpressionUsecase) EditorOperators() EditorOperators {
	if err := usecase.EnforceSecurity.Permission(models.SCENARIO_READ); err != nil {
		return EditorOperators{}
	}

	var operatorAccessors []ast.FuncAttributes
	for _, functionType := range ast.FuncOperators {
		operatorAccessors = append(operatorAccessors, ast.FuncAttributesMap[ast.Function(functionType)])
	}
	return EditorOperators{
		OperatorAccessors: operatorAccessors,
	}
}
