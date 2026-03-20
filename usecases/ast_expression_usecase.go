package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
)

type AstExpressionUsecaseRepository interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
	GetDataModel(
		ctx context.Context,
		exec repositories.Executor,
		organizationID uuid.UUID,
		fetchEnumValues bool,
		useCache bool,
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

func (usecase AstExpressionUsecase) EditorIdentifiers(ctx context.Context, scenarioId string) (EditorIdentifiers, error) {
	exec := usecase.executorFactory.NewExecutor()
	scenario, err := usecase.repository.GetScenarioById(ctx, exec, scenarioId)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
		return EditorIdentifiers{}, err
	}

	dataModel, err := usecase.repository.GetDataModel(ctx, exec, scenario.OrganizationId, false, true)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	databaseAccessors, err := models.GetLinkedDatabaseIdentifiers(scenario, dataModel)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	payloadAccessors, err := models.GetPayloadIdentifiers(scenario, dataModel)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	return EditorIdentifiers{
		PayloadAccessors:  payloadAccessors,
		DatabaseAccessors: databaseAccessors,
	}, nil
}
