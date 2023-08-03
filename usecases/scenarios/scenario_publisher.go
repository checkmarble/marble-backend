package scenarios

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/utils"
)

type ScenarioPublisher struct {
	ScenarioPublicationsRepository  repositories.ScenarioPublicationRepository
	ScenarioWriteRepository         repositories.ScenarioWriteRepository
	ScenarioIterationReadRepository repositories.ScenarioIterationReadRepository
	ValidateScenarioIteration       ValidateScenarioIteration
}

func (publisher *ScenarioPublisher) PublishOrUnpublishIteration(
	tx repositories.Transaction,
	scenarioAndIteration ScenarioAndIteration,
	publicationAction models.PublicationAction,
) ([]models.ScenarioPublication, error) {
	var scenarioPublications []models.ScenarioPublication

	organizationId := scenarioAndIteration.Scenario.OrganizationId
	scenariosId := scenarioAndIteration.Scenario.Id
	iterationId := scenarioAndIteration.Iteration.Id
	liveVersionId := scenarioAndIteration.Scenario.LiveVersionID

	switch publicationAction {
	case models.Unpublish:
		{
			if liveVersionId == nil || *liveVersionId != iterationId {
				return nil, fmt.Errorf("unable to unpublish: scenario iteration %s is not currently live %w", iterationId, models.BadParameterError)
			}

			if sps, err := publisher.unpublishOldIteration(tx, organizationId, scenariosId, &iterationId); err != nil {
				return nil, err
			} else {
				scenarioPublications = append(scenarioPublications, sps...)
			}
		}
	case models.Publish:
		{
			if liveVersionId != nil && *liveVersionId == iterationId {
				return []models.ScenarioPublication{}, nil
			}
			if err := ScenarioValidationToError(publisher.ValidateScenarioIteration.Validate(scenarioAndIteration)); err != nil {
				return nil, fmt.Errorf("Can't validate scenario %w %w", err, models.BadParameterError)
			}

			newVersion, err := publisher.getNewVersion(tx, organizationId, scenariosId)
			if err != nil {
				return nil, err
			}

			// FIXME Just temporarily placed here, will be moved to scenario iteration write repo
			err = publisher.ScenarioPublicationsRepository.UpdateScenarioIterationVersion(tx, iterationId, newVersion)
			if err != nil {
				return nil, err
			}

			if sps, err := publisher.unpublishOldIteration(tx, organizationId, scenariosId, liveVersionId); err != nil {
				return nil, err
			} else {
				scenarioPublications = append(scenarioPublications, sps...)
			}

			if sp, err := publisher.publishNewIteration(tx, organizationId, scenariosId, iterationId); err != nil {
				return nil, err
			} else {
				scenarioPublications = append(scenarioPublications, sp)
			}
		}
	}

	return scenarioPublications, nil
}

func (publisher *ScenarioPublisher) unpublishOldIteration(tx repositories.Transaction, organizationId, scenarioId string, liveVersionId *string) ([]models.ScenarioPublication, error) {
	if liveVersionId == nil {
		return []models.ScenarioPublication{}, nil
	}

	newScenarioPublicationId := utils.NewPrimaryKey(organizationId)
	if err := publisher.ScenarioPublicationsRepository.CreateScenarioPublication(tx, models.CreateScenarioPublicationInput{
		OrganizationId:      organizationId,
		ScenarioIterationId: *liveVersionId,
		ScenarioId:          scenarioId,
		PublicationAction:   models.Unpublish,
	}, newScenarioPublicationId); err != nil {
		return nil, err
	}

	if err := publisher.ScenarioWriteRepository.UpdateScenarioLiveItereationId(tx, scenarioId, nil); err != nil {
		return nil, err
	}
	scenarioPublication, err := publisher.ScenarioPublicationsRepository.GetScenarioPublicationById(tx, newScenarioPublicationId)
	return []models.ScenarioPublication{scenarioPublication}, err
}

func (publisher *ScenarioPublisher) publishNewIteration(tx repositories.Transaction, organizationId, scenarioId, scenarioIterationId string) (models.ScenarioPublication, error) {
	newScenarioPublicationId := utils.NewPrimaryKey(organizationId)
	if err := publisher.ScenarioPublicationsRepository.CreateScenarioPublication(tx, models.CreateScenarioPublicationInput{
		OrganizationId:      organizationId,
		ScenarioIterationId: scenarioIterationId,
		ScenarioId:          scenarioId,
		PublicationAction:   models.Publish,
	}, newScenarioPublicationId); err != nil {
		return models.ScenarioPublication{}, err
	}

	scenarioPublication, err := publisher.ScenarioPublicationsRepository.GetScenarioPublicationById(tx, newScenarioPublicationId)
	if err != nil {
		return models.ScenarioPublication{}, err
	}

	if err = publisher.ScenarioWriteRepository.UpdateScenarioLiveItereationId(tx, scenarioId, &scenarioIterationId); err != nil {
		return models.ScenarioPublication{}, err
	}
	return scenarioPublication, nil
}

func (publisher *ScenarioPublisher) getNewVersion(tx repositories.Transaction, organizationId, scenarioId string) (int, error) {
	scenarioIterations, err := publisher.ScenarioIterationReadRepository.ListScenarioIterations(tx, organizationId, models.GetScenarioIterationFilters{ScenarioId: &scenarioId})
	if err != nil {
		return 0, err
	}
	newVersion := 1
	for _, scenarioIteration := range scenarioIterations {
		if scenarioIteration.Version != nil {
			newVersion = *scenarioIteration.Version + 1
		}
	}
	return newVersion, nil
}
