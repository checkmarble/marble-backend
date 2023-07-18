package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/scenarios"
	"marble/marble-backend/usecases/security"
)

type ScenarioPublicationUsecase struct {
	transactionFactory              repositories.TransactionFactory
	scenarioPublicationsRepository  repositories.ScenarioPublicationRepository
	scenarioReadRepository          repositories.ScenarioReadRepository
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository
	OrganizationIdOfContext         string
	enforceSecurity                 security.EnforceSecurityScenario
	scenarioPublisher               scenarios.ScenarioPublisher
}

func (usecase *ScenarioPublicationUsecase) GetScenarioPublication(scenarioPublicationID string) (models.ScenarioPublication, error) {
	scenarioPublication, err := usecase.scenarioPublicationsRepository.GetScenarioPublicationById(nil, scenarioPublicationID)
	if err != nil {
		return models.ScenarioPublication{}, err
	}

	// Enforce permissions
	if err := usecase.enforceSecurity.ReadScenarioPublication(scenarioPublication); err != nil {
		return models.ScenarioPublication{}, err
	}
	return scenarioPublication, nil
}

func (usecase *ScenarioPublicationUsecase) ListScenarioPublications(filters models.ListScenarioPublicationsFilters) ([]models.ScenarioPublication, error) {
	// Enforce permissions
	if err := usecase.enforceSecurity.ListScenarios(usecase.OrganizationIdOfContext); err != nil {
		return nil, err
	}

	return usecase.scenarioPublicationsRepository.ListScenarioPublicationsOfOrganization(nil, usecase.OrganizationIdOfContext, filters)
}

func (usecase *ScenarioPublicationUsecase) ExecuteScenarioPublicationAction(ctx context.Context, input models.PublishScenarioIterationInput) ([]models.ScenarioPublication, error) {
	var scenarioPublications []models.ScenarioPublication
	err := usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		// FIXME Outside of transaction until the scenario iteration write repo is migrated
		scenarioIteration, err := usecase.scenarioIterationReadRepository.GetScenarioIteration(ctx, usecase.OrganizationIdOfContext, input.ScenarioIterationId)
		if err != nil {
			return err
		}

		scenario, err := usecase.scenarioReadRepository.GetScenarioById(tx, scenarioIteration.ScenarioID)
		if err != nil {
			return err
		}

		// Enforce permissions
		if err := usecase.enforceSecurity.PublishScenario(scenario); err != nil {
			return err
		}

		scenarioPublications, err = usecase.scenarioPublisher.PublishOrUnpublishIteration(tx, ctx, scenario.OrganizationID, input)
		return err
	})
	if err != nil {
		return nil, err
	}
	return scenarioPublications, nil
}
