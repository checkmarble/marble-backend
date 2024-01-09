package scenarios

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/analytics"
	"github.com/checkmarble/marble-backend/utils"
)

type ScenarioPublisherRepository interface {
	UpdateScenarioLiveIterationId(ctx context.Context, tx repositories.Transaction, scenarioId string, scenarioIterationId *string) error
	ListScenarioIterations(ctx context.Context, tx repositories.Transaction, organizationId string, filters models.GetScenarioIterationFilters) ([]models.ScenarioIteration, error)
	UpdateScenarioIterationVersion(ctx context.Context, tx repositories.Transaction, scenarioIterationId string, newVersion int) error
}

type ScenarioPublisher struct {
	Repository                     ScenarioPublisherRepository
	ValidateScenarioIteration      ValidateScenarioIteration
	ScenarioPublicationsRepository repositories.ScenarioPublicationRepository
}

func (publisher *ScenarioPublisher) PublishOrUnpublishIteration(
	ctx context.Context,
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

			if sps, err := publisher.unpublishOldIteration(ctx, tx, organizationId, scenariosId, &iterationId); err != nil {
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
			if err := ScenarioValidationToError(publisher.ValidateScenarioIteration.Validate(ctx, scenarioAndIteration)); err != nil {
				return nil, fmt.Errorf("can't validate scenario %w %w", err, models.BadParameterError)
			}

			scenarioVersion, err := publisher.getScenarioVersion(ctx, tx, organizationId, scenariosId, iterationId)
			if err != nil {
				return nil, err
			}

			err = publisher.Repository.UpdateScenarioIterationVersion(ctx, tx, iterationId, scenarioVersion)
			if err != nil {
				return nil, err
			}

			if sps, err := publisher.unpublishOldIteration(ctx, tx, organizationId, scenariosId, liveVersionId); err != nil {
				return nil, err
			} else {
				scenarioPublications = append(scenarioPublications, sps...)
			}

			if sp, err := publisher.publishNewIteration(ctx, tx, organizationId, scenariosId, iterationId); err != nil {
				return nil, err
			} else {
				scenarioPublications = append(scenarioPublications, sp)
			}

			analytics.TrackEvent(ctx, models.AnalyticsScenarioIterationPublished, map[string]interface{}{"scenario_iteration_id": iterationId})
		}
	}

	return scenarioPublications, nil
}

func (publisher *ScenarioPublisher) unpublishOldIteration(ctx context.Context, tx repositories.Transaction, organizationId, scenarioId string, liveVersionId *string) ([]models.ScenarioPublication, error) {
	if liveVersionId == nil {
		return []models.ScenarioPublication{}, nil
	}

	newScenarioPublicationId := utils.NewPrimaryKey(organizationId)
	if err := publisher.ScenarioPublicationsRepository.CreateScenarioPublication(ctx, tx, models.CreateScenarioPublicationInput{
		OrganizationId:      organizationId,
		ScenarioIterationId: *liveVersionId,
		ScenarioId:          scenarioId,
		PublicationAction:   models.Unpublish,
	}, newScenarioPublicationId); err != nil {
		return nil, err
	}

	if err := publisher.Repository.UpdateScenarioLiveIterationId(ctx, tx, scenarioId, nil); err != nil {
		return nil, err
	}
	scenarioPublication, err := publisher.ScenarioPublicationsRepository.GetScenarioPublicationById(ctx, tx, newScenarioPublicationId)
	return []models.ScenarioPublication{scenarioPublication}, err
}

func (publisher *ScenarioPublisher) publishNewIteration(ctx context.Context, tx repositories.Transaction, organizationId, scenarioId, scenarioIterationId string) (models.ScenarioPublication, error) {
	newScenarioPublicationId := utils.NewPrimaryKey(organizationId)
	if err := publisher.ScenarioPublicationsRepository.CreateScenarioPublication(ctx, tx, models.CreateScenarioPublicationInput{
		OrganizationId:      organizationId,
		ScenarioIterationId: scenarioIterationId,
		ScenarioId:          scenarioId,
		PublicationAction:   models.Publish,
	}, newScenarioPublicationId); err != nil {
		return models.ScenarioPublication{}, err
	}

	scenarioPublication, err := publisher.ScenarioPublicationsRepository.GetScenarioPublicationById(ctx, tx, newScenarioPublicationId)
	if err != nil {
		return models.ScenarioPublication{}, err
	}

	if err = publisher.Repository.UpdateScenarioLiveIterationId(ctx, tx, scenarioId, &scenarioIterationId); err != nil {
		return models.ScenarioPublication{}, err
	}
	return scenarioPublication, nil
}

func (publisher *ScenarioPublisher) getScenarioVersion(ctx context.Context, tx repositories.Transaction, organizationId, scenarioId, iterationId string) (int, error) {
	scenarioIterations, err := publisher.Repository.ListScenarioIterations(ctx, tx, organizationId, models.GetScenarioIterationFilters{ScenarioId: &scenarioId})
	if err != nil {
		return 0, err
	}

	for _, scenarioIteration := range scenarioIterations {
		if scenarioIteration.Id == iterationId && scenarioIteration.Version != nil {
			return *scenarioIteration.Version, nil
		}
	}

	var latestVersion int
	for _, scenarioIteration := range scenarioIterations {
		if scenarioIteration.Version != nil && *scenarioIteration.Version > latestVersion {
			latestVersion = *scenarioIteration.Version
		}
	}
	newVersion := latestVersion + 1

	return newVersion, nil
}
