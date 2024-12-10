package scenarios

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/tracking"
)

type ScenarioPublisherRepository interface {
	UpdateScenarioLiveIterationId(ctx context.Context, exec repositories.Executor, scenarioId string, scenarioIterationId *string) error
	ListScenarioIterations(ctx context.Context, exec repositories.Executor, organizationId string,
		filters models.GetScenarioIterationFilters) ([]models.ScenarioIteration, error)
	UpdateScenarioIterationVersion(ctx context.Context, exec repositories.Executor,
		scenarioIterationId string, newVersion int) error
}

type ScenarioPublisher struct {
	Repository                     ScenarioPublisherRepository
	ValidateScenarioIteration      ValidateScenarioIteration
	ScenarioPublicationsRepository repositories.ScenarioPublicationRepository
	ScenarioTestRunRepository      repositories.ScenarioTestRunRepository
}

func (publisher ScenarioPublisher) PublishOrUnpublishIteration(
	ctx context.Context,
	tx repositories.Transaction,
	scenarioAndIteration models.ScenarioAndIteration,
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

			if sps, err := publisher.unpublishOldIteration(ctx, tx, organizationId,
				scenariosId, &iterationId); err != nil {
				return nil, err
			} else {
				scenarioPublications = append(scenarioPublications, sps...)
			}
		}
	case models.Publish:
		{
			if scenarioAndIteration.Iteration.Version == nil {
				return nil, errors.Wrap(models.ErrScenarioIterationIsDraft,
					"input scenario iteration is a draft in PublishOrUnpublishIteration")
			}

			if liveVersionId != nil && *liveVersionId == iterationId {
				return []models.ScenarioPublication{}, nil
			}

			if err := ScenarioValidationToError(publisher.ValidateScenarioIteration.Validate(
				ctx, scenarioAndIteration)); err != nil {
				return nil, errors.Wrap(
					models.ErrScenarioIterationNotValid,
					fmt.Sprintf("Error validating scenario iteration %s: %s", iterationId, err.Error()),
				)
			}
			if sps, err := publisher.unpublishOldIteration(ctx, tx, organizationId,
				scenariosId, liveVersionId); err != nil {
				return nil, err
			} else {
				scenarioPublications = append(scenarioPublications, sps...)
			}

			if sp, err := publisher.publishNewIteration(ctx, tx, organizationId,
				scenariosId, iterationId); err != nil {
				return nil, err
			} else {
				scenarioPublications = append(scenarioPublications, sp)
			}

			tracking.TrackEvent(ctx, models.AnalyticsScenarioIterationPublished, map[string]interface{}{
				"scenario_iteration_id": iterationId,
			})
		}
	default:
		return nil, errors.Wrap(
			models.BadParameterError,
			"unknown publication action: "+publicationAction.String(),
		)
	}

	return scenarioPublications, nil
}

func (publisher ScenarioPublisher) shutDownTestRunIfNeeded(ctx context.Context, tx repositories.Transaction, liveVersionId string) error {
	testrun, errTestrun := publisher.ScenarioTestRunRepository.GetTestRunByLiveVersionID(ctx, tx, liveVersionId)
	if errTestrun != nil {
		return errTestrun
	}
	if testrun != nil && (testrun.Status == models.Up || testrun.Status == models.Pending) {
		return publisher.ScenarioTestRunRepository.UpdateTestRunStatus(ctx, tx, testrun.Id, models.Down)
	}

	return nil
}

func (publisher ScenarioPublisher) unpublishOldIteration(
	ctx context.Context, tx repositories.Transaction, organizationId, scenarioId string, liveVersionId *string,
) ([]models.ScenarioPublication, error) {
	if liveVersionId == nil {
		return []models.ScenarioPublication{}, nil
	}

	newScenarioPublicationId := pure_utils.NewPrimaryKey(organizationId)
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
	// stop any running testrun scenario associated to
	if errShutdown := publisher.shutDownTestRunIfNeeded(ctx, tx, *liveVersionId); errShutdown != nil {
		return nil, errShutdown
	}
	scenarioPublication, err := publisher.ScenarioPublicationsRepository.GetScenarioPublicationById(ctx, tx, newScenarioPublicationId)
	return []models.ScenarioPublication{scenarioPublication}, err
}

func (publisher ScenarioPublisher) publishNewIteration(ctx context.Context,
	tx repositories.Transaction, organizationId, scenarioId, scenarioIterationId string,
) (models.ScenarioPublication, error) {
	newScenarioPublicationId := pure_utils.NewPrimaryKey(organizationId)
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
	// stop any running testrun scenario associated to
	if errShutdown := publisher.shutDownTestRunIfNeeded(ctx, tx, scenarioIterationId); errShutdown != nil {
		return models.ScenarioPublication{}, errShutdown
	}

	return scenarioPublication, nil
}
