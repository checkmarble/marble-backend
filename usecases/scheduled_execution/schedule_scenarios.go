package scheduled_execution

import (
	"context"
	"fmt"
	"time"

	"github.com/adhocore/gronx"
	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
)

func (usecase *RunScheduledExecution) ScheduleScenarioIfDue(ctx context.Context, organizationId string, scenarioId string) error {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	scenario, err := usecase.repository.GetScenarioById(ctx, exec, scenarioId)
	if err != nil {
		return err
	}

	publishedVersion, err := usecase.getPublishedScenarioIteration(ctx, exec, scenario)
	if err != nil {
		return err
	}
	if publishedVersion == nil {
		logger.DebugContext(ctx, fmt.Sprintf("scenario %s has no published version", scenarioId))
		return nil
	}

	previousExecutions, err := usecase.repository.ListScheduledExecutions(ctx, exec, models.ListScheduledExecutionsFilters{
		ScenarioId: scenarioId, Status: []models.ScheduledExecutionStatus{
			models.ScheduledExecutionPending, models.ScheduledExecutionProcessing,
		},
	})
	if err != nil {
		return err
	}
	if len(previousExecutions) > 0 {
		logger.DebugContext(ctx, fmt.Sprintf("scenario %s already has a pending or processing scheduled execution", scenarioId))
		return nil
	}

	isDue, err := usecase.scenarioIsDue(ctx, *publishedVersion, scenario)
	if err != nil {
		return err
	}
	if !isDue {
		return nil
	}

	logger.DebugContext(ctx, fmt.Sprintf("Scenario iteration %s is due", publishedVersion.Id))
	scheduledExecutionId := pure_utils.NewPrimaryKey(organizationId)
	return usecase.repository.CreateScheduledExecution(ctx, exec, models.CreateScheduledExecutionInput{
		OrganizationId:      organizationId,
		ScenarioId:          scenarioId,
		ScenarioIterationId: publishedVersion.Id,
		Manual:              false,
	}, scheduledExecutionId)
}

func (usecase *RunScheduledExecution) scenarioIsDue(
	ctx context.Context,
	publishedVersion models.PublishedScenarioIteration,
	scenario models.Scenario,
) (bool, error) {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	if publishedVersion.Body.Schedule == "" {
		logger.DebugContext(ctx, fmt.Sprintf("Scenario iteration %s has no schedule", publishedVersion.Id))
		return false, nil
	}
	gron := gronx.New()
	ok := gron.IsValid(publishedVersion.Body.Schedule)
	if !ok {
		return false, fmt.Errorf("invalid schedule: %w", models.BadParameterError)
	}

	previousExecutions, err := usecase.repository.ListScheduledExecutions(ctx, exec, models.ListScheduledExecutionsFilters{
		ScenarioId: scenario.Id, ExcludeManual: true,
	})
	if err != nil {
		return false, fmt.Errorf("error listing scheduled executions: %w", err)
	}

	publications, err := usecase.scenarioPublicationsRepository.ListScenarioPublicationsOfOrganization(
		ctx, exec, scenario.OrganizationId, models.ListScenarioPublicationsFilters{ScenarioId: &scenario.Id})
	if err != nil {
		return false, err
	}

	tz, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		return false, errors.Wrap(err, "error loading timezone")
	}
	return executionIsDueNow(publishedVersion.Body.Schedule, previousExecutions, publications, tz)
}

func executionIsDueNow(
	schedule string,
	previousExecutions []models.ScheduledExecution,
	publications []models.ScenarioPublication,
	tz *time.Location,
) (bool, error) {
	if tz == nil {
		return false, errors.New("Nil timezone passed in executionIsDueNow")
	}
	var referenceTime time.Time
	if len(previousExecutions) > 0 {
		referenceTime = previousExecutions[0].StartedAt.In(tz)
	} else {
		// if there is no previous execution, consider the last iteration publication time to be the last execution time
		referenceTime = publications[0].CreatedAt.In(tz)
	}

	nextTick, err := gronx.NextTickAfter(schedule, referenceTime, false)
	if err != nil {
		return true, err
	}
	if nextTick.After(time.Now()) {
		return false, nil
	}
	return true, nil
}

func (usecase *RunScheduledExecution) getPublishedScenarioIteration(
	ctx context.Context,
	exec repositories.Executor,
	scenario models.Scenario,
) (*models.PublishedScenarioIteration, error) {
	if scenario.LiveVersionID == nil {
		return nil, nil
	}

	liveVersion, err := usecase.repository.GetScenarioIteration(ctx, exec, *scenario.LiveVersionID)
	if err != nil {
		return nil, err
	}
	publishedVersion := models.NewPublishedScenarioIteration(liveVersion)
	return &publishedVersion, nil
}
