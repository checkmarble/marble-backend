package billing

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
)

type lagoBillingWorkerRepository interface {
	SendEvent(ctx context.Context, event models.BillingEvent) error
}

type SendLagoBillingEventsWorker struct {
	river.WorkerDefaults[models.SendBillingEventArgs]

	repository lagoBillingWorkerRepository
}

func NewSendLagoBillingEventsWorker(repository lagoBillingWorkerRepository) *SendLagoBillingEventsWorker {
	return &SendLagoBillingEventsWorker{
		repository: repository,
	}
}

func (w *SendLagoBillingEventsWorker) Timeout(job *river.Job[models.SendBillingEventArgs]) time.Duration {
	return 10 * time.Second
}

func (w *SendLagoBillingEventsWorker) Work(ctx context.Context, job *river.Job[models.SendBillingEventArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Sending billing events", "events", job.Args.Event)

	if err := w.repository.SendEvent(ctx, job.Args.Event); err != nil {
		return err
	}

	logger.DebugContext(ctx, "Billing events sent", "events", job.Args.Event)

	return nil
}
