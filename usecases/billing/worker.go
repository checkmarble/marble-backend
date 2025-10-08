package billing

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
)

type billingWorkerRepository interface {
	SendEvents(ctx context.Context, events []models.BillingEvent) error
}

type SendBillingEventsWorker struct {
	river.WorkerDefaults[models.SendBillingEventsArgs]

	repository billingWorkerRepository
}

func NewSendBillingEventsWorker(repository billingWorkerRepository) *SendBillingEventsWorker {
	return &SendBillingEventsWorker{
		repository: repository,
	}
}

func (w *SendBillingEventsWorker) Timeout(job *river.Job[models.SendBillingEventsArgs]) time.Duration {
	return 10 * time.Second
}

func (w *SendBillingEventsWorker) Work(ctx context.Context, job *river.Job[models.SendBillingEventsArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Sending billing events", "events", job.Args.Events)

	if err := w.repository.SendEvents(ctx, job.Args.Events); err != nil {
		return err
	}

	logger.DebugContext(ctx, "Billing events sent", "events", job.Args.Events)

	return nil
}
