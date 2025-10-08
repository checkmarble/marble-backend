package billing

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
)

type billingWorkerRepository interface {
	SendEvent(ctx context.Context, event models.BillingEvent) error
}

type SendBillingEventsWorker struct {
	river.WorkerDefaults[models.SendBillingEventArgs]

	repository billingWorkerRepository
}

func NewSendBillingEventsWorker(repository billingWorkerRepository) *SendBillingEventsWorker {
	return &SendBillingEventsWorker{
		repository: repository,
	}
}

func (w *SendBillingEventsWorker) Timeout(job *river.Job[models.SendBillingEventArgs]) time.Duration {
	return 10 * time.Second
}

func (w *SendBillingEventsWorker) Work(ctx context.Context, job *river.Job[models.SendBillingEventArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Sending billing events", "events", job.Args.Event)

	if err := w.repository.SendEvent(ctx, job.Args.Event); err != nil {
		return err
	}

	logger.DebugContext(ctx, "Billing events sent", "events", job.Args.Event)

	return nil
}
