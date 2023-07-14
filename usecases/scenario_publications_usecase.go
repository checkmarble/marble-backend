package usecases

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/security"
	"marble/marble-backend/utils"
)

type ScenarioPublicationUsecase struct {
	transactionFactory              repositories.TransactionFactory
	scenarioPublicationsRepository  repositories.ScenarioPublicationRepository
	scenarioReadRepository          repositories.ScenarioReadRepository
	scenarioWriteRepository         repositories.ScenarioWriteRepository
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository
	OrganizationIdOfContext         string
	enforceSecurity                 security.EnforceSecurityScenario
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

		switch input.PublicationAction {
		case models.Unpublish:
			{
				if scenario.LiveVersionID == nil || *scenario.LiveVersionID != input.ScenarioIterationId {
					return fmt.Errorf("unable to unpublish: scenario iteration %s is not currently live %w", input.ScenarioIterationId, models.BadParameterError)
				}

				if sp, err := usecase.unpublishOldIteration(tx, usecase.OrganizationIdOfContext, scenarioIteration.ScenarioID, input.ScenarioIterationId); err != nil {
					return err
				} else {
					scenarioPublications = append(scenarioPublications, sp)
				}
			}
		case models.Publish:
			{
				if scenario.LiveVersionID != nil && *scenario.LiveVersionID == input.ScenarioIterationId {
					return nil
				}
				if err := scenarioIteration.IsValidForPublication(); err != nil {
					return err
				}

				var newVersion int
				if scenario.LiveVersionID == nil {
					if scenario.LiveVersionID == nil {
						newVersion = 1
					} else {
						// FIXME Outside of transaction until the scenario iteration write repo is migrated
						currentScenarioIteration, err := usecase.scenarioIterationReadRepository.GetScenarioIteration(ctx, usecase.OrganizationIdOfContext, *scenario.LiveVersionID)
						if err != nil {
							return err
						}
						newVersion = *currentScenarioIteration.Version + 1
					}

				}
				// FIXME Just temporarily placed here, will be moved to scenario iteration write repo
				err := usecase.scenarioPublicationsRepository.UpdateScenarioIterationVersion(tx, input.ScenarioIterationId, newVersion)
				if err != nil {
					return err
				}

				if sp, err := usecase.unpublishOldIteration(tx, usecase.OrganizationIdOfContext, scenarioIteration.ScenarioID, *scenario.LiveVersionID); err != nil {
					return err
				} else {
					scenarioPublications = append(scenarioPublications, sp)
				}

				if sp, err := usecase.publishNewIteration(tx, usecase.OrganizationIdOfContext, scenarioIteration.ScenarioID, input.ScenarioIterationId); err != nil {
					return err
				} else {
					scenarioPublications = append(scenarioPublications, sp)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return scenarioPublications, nil
}

func (usecase *ScenarioPublicationUsecase) unpublishOldIteration(tx repositories.Transaction, organizationId, scenarioId, scenarioIterationId string) (models.ScenarioPublication, error) {
	newScenarioPublicationId := utils.NewPrimaryKey(organizationId)
	if err := usecase.scenarioPublicationsRepository.CreateScenarioPublication(tx, models.CreateScenarioPublicationInput{
		OrganizationId:      organizationId,
		ScenarioIterationId: scenarioIterationId,
		ScenarioId:          scenarioId,
		PublicationAction:   models.Unpublish,
	}, newScenarioPublicationId); err != nil {
		return models.ScenarioPublication{}, err
	}

	scenarioPublication, err := usecase.scenarioPublicationsRepository.GetScenarioPublicationById(tx, newScenarioPublicationId)
	if err != nil {
		return models.ScenarioPublication{}, err
	}

	if err = usecase.scenarioWriteRepository.UpdateScenarioLiveItereationId(tx, scenarioId, nil); err != nil {
		return models.ScenarioPublication{}, err
	}
	return scenarioPublication, nil
}

func (usecase *ScenarioPublicationUsecase) publishNewIteration(tx repositories.Transaction, organizationId, scenarioId, scenarioIterationId string) (models.ScenarioPublication, error) {
	newScenarioPublicationId := utils.NewPrimaryKey(organizationId)
	if err := usecase.scenarioPublicationsRepository.CreateScenarioPublication(tx, models.CreateScenarioPublicationInput{
		OrganizationId:      organizationId,
		ScenarioIterationId: scenarioIterationId,
		ScenarioId:          scenarioId,
		PublicationAction:   models.Publish,
	}, newScenarioPublicationId); err != nil {
		return models.ScenarioPublication{}, err
	}

	scenarioPublication, err := usecase.scenarioPublicationsRepository.GetScenarioPublicationById(tx, newScenarioPublicationId)
	if err != nil {
		return models.ScenarioPublication{}, err
	}

	if err = usecase.scenarioWriteRepository.UpdateScenarioLiveItereationId(tx, scenarioId, &scenarioIterationId); err != nil {
		return models.ScenarioPublication{}, err
	}
	return scenarioPublication, nil
}
