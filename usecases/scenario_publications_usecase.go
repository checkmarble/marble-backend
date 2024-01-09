package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
)

type ScenarioPublicationUsecase struct {
	transactionFactory             transaction.TransactionFactory
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
	OrganizationIdOfContext        func() (string, error)
	enforceSecurity                security.EnforceSecurityScenario
	scenarioFetcher                scenarios.ScenarioFetcher
	scenarioPublisher              scenarios.ScenarioPublisher
}

func (usecase *ScenarioPublicationUsecase) GetScenarioPublication(ctx context.Context, scenarioPublicationID string) (models.ScenarioPublication, error) {
	scenarioPublication, err := usecase.scenarioPublicationsRepository.GetScenarioPublicationById(ctx, nil, scenarioPublicationID)
	if err != nil {
		return models.ScenarioPublication{}, err
	}

	// Enforce permissions
	if err := usecase.enforceSecurity.ReadScenarioPublication(scenarioPublication); err != nil {
		return models.ScenarioPublication{}, err
	}
	return scenarioPublication, nil
}

func (usecase *ScenarioPublicationUsecase) ListScenarioPublications(ctx context.Context, filters models.ListScenarioPublicationsFilters) ([]models.ScenarioPublication, error) {
	organizationId, err := usecase.OrganizationIdOfContext()
	if err != nil {
		return nil, err
	}

	// Enforce permissions
	if err := usecase.enforceSecurity.ListScenarios(organizationId); err != nil {
		return nil, err
	}

	return usecase.scenarioPublicationsRepository.ListScenarioPublicationsOfOrganization(ctx, nil, organizationId, filters)
}

func (usecase *ScenarioPublicationUsecase) ExecuteScenarioPublicationAction(ctx context.Context, input models.PublishScenarioIterationInput) ([]models.ScenarioPublication, error) {
	return transaction.TransactionReturnValue(ctx, usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) ([]models.ScenarioPublication, error) {

		scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, input.ScenarioIterationId)
		if err != nil {
			return []models.ScenarioPublication{}, err
		}

		if err := usecase.enforceSecurity.PublishScenario(scenarioAndIteration.Scenario); err != nil {
			return []models.ScenarioPublication{}, err
		}

		return usecase.scenarioPublisher.PublishOrUnpublishIteration(ctx, tx, scenarioAndIteration, input.PublicationAction)
	})

}
