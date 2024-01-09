package usecases

import (
	"context"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type AstExpressionUsecaseRepository interface {
	GetScenarioById(ctx context.Context, tx repositories.Transaction, scenarioId string) (models.Scenario, error)
}

type AstExpressionUsecase struct {
	EnforceSecurity     security.EnforceSecurityScenario
	DataModelRepository repositories.DataModelRepository
	Repository          AstExpressionUsecaseRepository
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
		return nil, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}

	var visited []string
	recursiveDatabaseAccessor = func(path []string, links map[models.LinkName]models.LinkToSingle) error {
		for linkName, link := range links {
			table, found := dataModel.Tables[link.LinkedTableName]
			if !found {
				return fmt.Errorf("table %s not found in data model", scenario.TriggerObjectType)
			}

			relation := fmt.Sprintf("%s/%s", table.Name, linkName)
			idx := slices.Index(visited, relation)
			if idx != -1 {
				continue
			}
			visited = append(visited, relation)
			pathForLink := append(path, string(linkName))

			for fieldName := range table.Fields {
				dataAccessors = append(dataAccessors,
					ast.NewNodeDatabaseAccess(
						scenario.TriggerObjectType,
						string(fieldName),
						pathForLink,
					),
				)
			}

			if err := recursiveDatabaseAccessor(pathForLink, table.LinksToSingle); err != nil {
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

func (usecase *AstExpressionUsecase) EditorIdentifiers(ctx context.Context, scenarioId string) (EditorIdentifiers, error) {

	scenario, err := usecase.Repository.GetScenarioById(ctx, nil, scenarioId)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	if err := usecase.EnforceSecurity.ReadScenario(scenario); err != nil {
		return EditorIdentifiers{}, err
	}

	dataModel, err := usecase.DataModelRepository.GetDataModel(ctx, scenario.OrganizationId, false)
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
		operatorAccessors = append(operatorAccessors, ast.FuncAttributesMap[functionType])
	}
	return EditorOperators{
		OperatorAccessors: operatorAccessors,
	}
}
