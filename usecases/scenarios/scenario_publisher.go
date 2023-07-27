package scenarios

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/utils"
)

type ScenarioPublisher struct {
	scenarioPublicationsRepository  repositories.ScenarioPublicationRepository
	scenarioReadRepository          repositories.ScenarioReadRepository
	scenarioWriteRepository         repositories.ScenarioWriteRepository
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository
}

func NewScenarioPublisher(
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository,
	scenarioReadRepository repositories.ScenarioReadRepository,
	scenarioWriteRepository repositories.ScenarioWriteRepository,
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository,
) ScenarioPublisher {
	return ScenarioPublisher{
		scenarioPublicationsRepository:  scenarioPublicationsRepository,
		scenarioReadRepository:          scenarioReadRepository,
		scenarioWriteRepository:         scenarioWriteRepository,
		scenarioIterationReadRepository: scenarioIterationReadRepository,
	}
}

func (publisher *ScenarioPublisher) PublishOrUnpublishIteration(tx repositories.Transaction, organizationId string, input models.PublishScenarioIterationInput) ([]models.ScenarioPublication, error) {
	var scenarioPublications []models.ScenarioPublication

	scenarioIteration, err := publisher.scenarioIterationReadRepository.GetScenarioIteration(tx, input.ScenarioIterationId)
	if err != nil {
		return nil, err
	}

	scenario, err := publisher.scenarioReadRepository.GetScenarioById(tx, scenarioIteration.ScenarioID)
	if err != nil {
		return nil, err
	}

	switch input.PublicationAction {
	case models.Unpublish:
		{
			if scenario.LiveVersionID == nil || *scenario.LiveVersionID != input.ScenarioIterationId {
				return nil, fmt.Errorf("unable to unpublish: scenario iteration %s is not currently live %w", input.ScenarioIterationId, models.BadParameterError)
			}

			if sps, err := publisher.unpublishOldIteration(tx, organizationId, scenarioIteration.ScenarioID, &input.ScenarioIterationId); err != nil {
				return nil, err
			} else {
				scenarioPublications = append(scenarioPublications, sps...)
			}
		}
	case models.Publish:
		{
			if scenario.LiveVersionID != nil && *scenario.LiveVersionID == input.ScenarioIterationId {
				return []models.ScenarioPublication{}, nil
			}
			if err := scenarioIteration.IsValidForPublication(); err != nil {
				return nil, err
			}

			newVersion, err := publisher.getNewVersion(tx, organizationId, scenarioIteration.ScenarioID)
			if err != nil {
				return nil, err
			}

			// FIXME Just temporarily placed here, will be moved to scenario iteration write repo
			err = publisher.scenarioPublicationsRepository.UpdateScenarioIterationVersion(tx, input.ScenarioIterationId, newVersion)
			if err != nil {
				return nil, err
			}

			if sps, err := publisher.unpublishOldIteration(tx, organizationId, scenarioIteration.ScenarioID, scenario.LiveVersionID); err != nil {
				return nil, err
			} else {
				scenarioPublications = append(scenarioPublications, sps...)
			}

			if sp, err := publisher.publishNewIteration(tx, organizationId, scenarioIteration.ScenarioID, input.ScenarioIterationId); err != nil {
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
	if err := publisher.scenarioPublicationsRepository.CreateScenarioPublication(tx, models.CreateScenarioPublicationInput{
		OrganizationId:      organizationId,
		ScenarioIterationId: *liveVersionId,
		ScenarioId:          scenarioId,
		PublicationAction:   models.Unpublish,
	}, newScenarioPublicationId); err != nil {
		return nil, err
	}

	if err := publisher.scenarioWriteRepository.UpdateScenarioLiveItereationId(tx, scenarioId, nil); err != nil {
		return nil, err
	}
	scenarioPublication, err := publisher.scenarioPublicationsRepository.GetScenarioPublicationById(tx, newScenarioPublicationId)
	return []models.ScenarioPublication{scenarioPublication}, err
}

func (publisher *ScenarioPublisher) publishNewIteration(tx repositories.Transaction, organizationId, scenarioId, scenarioIterationId string) (models.ScenarioPublication, error) {
	newScenarioPublicationId := utils.NewPrimaryKey(organizationId)
	if err := publisher.scenarioPublicationsRepository.CreateScenarioPublication(tx, models.CreateScenarioPublicationInput{
		OrganizationId:      organizationId,
		ScenarioIterationId: scenarioIterationId,
		ScenarioId:          scenarioId,
		PublicationAction:   models.Publish,
	}, newScenarioPublicationId); err != nil {
		return models.ScenarioPublication{}, err
	}

	scenarioPublication, err := publisher.scenarioPublicationsRepository.GetScenarioPublicationById(tx, newScenarioPublicationId)
	if err != nil {
		return models.ScenarioPublication{}, err
	}

	if err = publisher.scenarioWriteRepository.UpdateScenarioLiveItereationId(tx, scenarioId, &scenarioIterationId); err != nil {
		return models.ScenarioPublication{}, err
	}
	return scenarioPublication, nil
}

func (publisher *ScenarioPublisher) getNewVersion(tx repositories.Transaction, organizationId, scenarioId string) (int, error) {
	scenarioIterations, err := publisher.scenarioIterationReadRepository.ListScenarioIterations(tx, organizationId, models.GetScenarioIterationFilters{ScenarioID: &scenarioId})
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
