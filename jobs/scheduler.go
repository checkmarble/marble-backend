package jobs

import (
	"context"

	"github.com/adhocore/gronx/pkg/tasker"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func errToReturnCode(err error) int {
	if err != nil {
		return 1
	}
	return 0
}

func RunScheduler(ctx context.Context, usecases usecases.Usecases) {
	taskr := tasker.New(tasker.Option{
		Verbose: true,
		Tz:      "Europe/Paris",
	}).WithContext(ctx)

	notConcurrent := false
	taskr.Task("* * * * *", func(ctx context.Context) (int, error) {
		logger := utils.LoggerFromContext(ctx).With("job", "schedule_due_scenarios")
		ctx = utils.StoreLoggerInContext(ctx, logger)
		err := ScheduleDueScenarios(ctx, usecases)
		return errToReturnCode(err), err
	}, notConcurrent)

	taskr.Task("* * * * *", func(ctx context.Context) (int, error) {
		logger := utils.LoggerFromContext(ctx).With("job", "execute_all_scheduled_scenarios")
		ctx = utils.StoreLoggerInContext(ctx, logger)
		err := ExecuteAllScheduledScenarios(ctx, usecases)
		return errToReturnCode(err), err
	})

	taskr.Task("* * * * *", func(ctx context.Context) (int, error) {
		logger := utils.LoggerFromContext(ctx).With("job", "ingest_data_from_csv")
		ctx = utils.StoreLoggerInContext(ctx, logger)
		err := IngestDataFromCsv(ctx, usecases)
		return errToReturnCode(err), err
	})

	taskr.Task("*/10 * * * *", func(ctx context.Context) (int, error) {
		logger := utils.LoggerFromContext(ctx).With("job", "send_webhook_events_to_convoy")
		ctx = utils.StoreLoggerInContext(ctx, logger)
		err := SendPendingWebhookEvents(ctx, usecases)
		return errToReturnCode(err), err
	})

	taskr.Run()
}
