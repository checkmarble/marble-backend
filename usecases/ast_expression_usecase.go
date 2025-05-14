package usecases

import (
	"context"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type AstExpressionUsecaseRepository interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
	GetDataModel(
		ctx context.Context,
		exec repositories.Executor,
		organizationID string,
		fetchEnumValues bool,
	) (models.DataModel, error)
}

type AstExpressionUsecase struct {
	executorFactory executor_factory.ExecutorFactory
	enforceSecurity security.EnforceSecurityScenario
	repository      AstExpressionUsecaseRepository
}

func NewAstExpressionUsecase(
	executorFactory executor_factory.ExecutorFactory,
	enforceSecurity security.EnforceSecurityScenario,
	repository AstExpressionUsecaseRepository,
) AstExpressionUsecase {
	return AstExpressionUsecase{
		executorFactory: executorFactory,
		enforceSecurity: enforceSecurity,
		repository:      repository,
	}
}

type EditorIdentifiers struct {
	PayloadAccessors  []ast.Node `json:"payload_accessors"`
	DatabaseAccessors []ast.Node `json:"database_accessors"`
}

func getLinkedDatabaseIdentifiers(scenario models.Scenario, dataModel models.DataModel) ([]ast.Node, error) {
	dataAccessors := []ast.Node{}
	var recursiveDatabaseAccessor func(
		baseTable string,
		path []string,
		links map[string]models.LinkToSingle,
	) error

	triggerObjectTable, found := dataModel.Tables[scenario.TriggerObjectType]
	if !found {
		return nil, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}

	var visited []string
	recursiveDatabaseAccessor = func(
		baseTable string,
		path []string,
		links map[string]models.LinkToSingle,
	) error {
		for linkName, link := range links {
			table, found := dataModel.Tables[link.ParentTableName]
			if !found {
				return fmt.Errorf("table %s not found in data model", scenario.TriggerObjectType)
			}

			relation := fmt.Sprintf("%s/%s", baseTable, linkName)
			idx := slices.Index(visited, relation)
			if idx != -1 {
				continue
			}
			visited = append(visited, relation)

			// deepcopy so that different identifiers don't collide
			pathForLink := append(make([]string, 0, len(path)+1), path...)
			pathForLink = append(pathForLink, linkName)

			for fieldName := range table.Fields {
				dataAccessors = append(dataAccessors,
					ast.NewNodeDatabaseAccess(
						scenario.TriggerObjectType,
						fieldName,
						pathForLink,
					),
				)
			}

			if err := recursiveDatabaseAccessor(table.Name, pathForLink, table.LinksToSingle); err != nil {
				return err
			}
		}
		return nil
	}

	if err := recursiveDatabaseAccessor(
		triggerObjectTable.Name,
		[]string{},
		triggerObjectTable.LinksToSingle,
	); err != nil {
		return nil, err
	}
	return dataAccessors, nil
}

func getPayloadIdentifiers(scenario models.Scenario, dataModel models.DataModel) ([]ast.Node, error) {
	dataAccessors := []ast.Node{}

	triggerObjectTable, found := dataModel.Tables[scenario.TriggerObjectType]
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

func (usecase AstExpressionUsecase) EditorIdentifiers(ctx context.Context, scenarioId string) (EditorIdentifiers, error) {
	exec := usecase.executorFactory.NewExecutor()
	scenario, err := usecase.repository.GetScenarioById(ctx, exec, scenarioId)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
		return EditorIdentifiers{}, err
	}

	dataModel, err := usecase.repository.GetDataModel(ctx, exec, scenario.OrganizationId, false)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	databaseAccessors, err := getLinkedDatabaseIdentifiers(scenario, dataModel)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	payloadAccessors, err := getPayloadIdentifiers(scenario, dataModel)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	return EditorIdentifiers{
		PayloadAccessors:  payloadAccessors,
		DatabaseAccessors: databaseAccessors,
	}, nil
}
