package scheduled_execution

import (
	"context"
	"fmt"
	"time"

	"github.com/adhocore/gronx"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
)

func (usecase *RunScheduledExecution) ScheduleScenarioIfDue(ctx context.Context, scenario models.Scenario) (bool, error) {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)

	publishedVersion, err := usecase.getPublishedScenarioIteration(ctx, exec, scenario)
	if err != nil {
		return false, err
	}
	if publishedVersion == nil {
		logger.DebugContext(ctx, fmt.Sprintf(`scenario "%s" has no published version`, scenario.Name))
		return false, nil
	}

	previousExecutions, err := usecase.repository.ListScheduledExecutions(
		ctx,
		exec,
		models.ListScheduledExecutionsFilters{
			OrganizationId: scenario.OrganizationId,
			ScenarioId:     scenario.Id,
		},
		nil,
	)
	if err != nil {
		return false, err
	}
	for _, ex := range previousExecutions {
		if ex.Status == models.ScheduledExecutionPending ||
			ex.Status == models.ScheduledExecutionProcessing {
			logger.DebugContext(ctx, fmt.Sprintf(`scenario "%s" already has a pending or processing scheduled execution`, scenario.Name))
			return false, nil
		}
	}

	isDue, err := usecase.scenarioIsDue(ctx, *publishedVersion, scenario, previousExecutions)
	if err != nil {
		return false, err
	}
	if !isDue {
		return false, nil
	}

	logger.DebugContext(ctx,
		fmt.Sprintf(`Version %d of scenario "%s" is due and will be scheduled`,
			publishedVersion.Version, scenario.Name))

	scheduledExecutionId := uuid.Must(uuid.NewV7()).String()
	return true, usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.repository.CreateScheduledExecution(ctx, tx, models.CreateScheduledExecutionInput{
			OrganizationId:      scenario.OrganizationId,
			ScenarioId:          scenario.Id,
			ScenarioIterationId: publishedVersion.Id,
			Manual:              false,
		}, scheduledExecutionId); err != nil {
			return err
		}
		return usecase.taskQueueRepository.EnqueueScheduledExecutionTask(ctx, tx,
			scenario.OrganizationId, scheduledExecutionId)
	})
}

func (usecase *RunScheduledExecution) scenarioIsDue(
	ctx context.Context,
	publishedVersion models.PublishedScenarioIteration,
	scenario models.Scenario,
	previousExecutions []models.ScheduledExecution,
) (bool, error) {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	if publishedVersion.Body.Schedule == "" {
		logger.DebugContext(ctx, fmt.Sprintf("Scenario iteration %d has no schedule", publishedVersion.Version))
		return false, nil
	}
	gron := gronx.New()
	ok := gron.IsValid(publishedVersion.Body.Schedule)
	if !ok {
		return false, errors.Wrapf(models.BadParameterError, `Bad schedule format: "%s"`, publishedVersion.Body.Schedule)
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
	if tz == nil {
		return false, errors.New("Nil timezone passed in executionIsDueNow")
	}

	nonManualExecutions := utils.Filter(previousExecutions,
		func(e models.ScheduledExecution) bool { return !e.Manual })

	var referenceTime time.Time
	if len(nonManualExecutions) > 0 {
		referenceTime = nonManualExecutions[0].StartedAt.In(tz)
	} else {
		// if there is no previous execution, consider the last iteration publication time to be the last execution time
		referenceTime = publications[0].CreatedAt.In(tz)
	}

	nextTick, err := gronx.NextTickAfter(publishedVersion.Body.Schedule, referenceTime, false)
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

	liveVersion, err := usecase.repository.GetScenarioIteration(ctx, exec, *scenario.LiveVersionID, false)
	if err != nil {
		return nil, err
	}
	publishedVersion := models.NewPublishedScenarioIteration(liveVersion)
	return &publishedVersion, nil
}
