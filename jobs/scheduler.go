package jobs

import (
	"context"

	"github.com/adhocore/gronx/pkg/tasker"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

// Deprecated and to be moved into the river task scheduler
func RunScheduler(ctx context.Context, usecases usecases.Usecaser) {
	taskr := tasker.New(tasker.Option{
		Verbose: true,
		Tz:      "Europe/Paris",
	}).WithContext(ctx)

	taskr.Task("* * * * *", func(ctx context.Context) (int, error) {
		logger := utils.LoggerFromContext(ctx).With("job", "execute_all_scheduled_scenarios")
		ctx = utils.StoreLoggerInContext(ctx, logger)
		ExecuteAllScheduledScenarios(ctx, usecases)
		return 0, nil
	})

	taskr.Task("* * * * *", func(ctx context.Context) (int, error) {
		logger := utils.LoggerFromContext(ctx).With("job", "ingest_data_from_csv")
		ctx = utils.StoreLoggerInContext(ctx, logger)
		IngestDataFromCsv(ctx, usecases)
		return 0, nil
	})

	taskr.Task("*/10 * * * *", func(ctx context.Context) (int, error) {
		logger := utils.LoggerFromContext(ctx).With("job", "send_webhook_events_to_convoy")
		ctx = utils.StoreLoggerInContext(ctx, logger)
		SendPendingWebhookEvents(ctx, usecases)
		return 0, nil
	})

	taskr.Run()
}
