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

type SendLagoBillingEventWorker struct {
	river.WorkerDefaults[models.SendBillingEventArgs]

	repository lagoBillingWorkerRepository
}

func NewSendLagoBillingEventWorker(repository lagoBillingWorkerRepository) *SendLagoBillingEventWorker {
	return &SendLagoBillingEventWorker{
		repository: repository,
	}
}

func (w *SendLagoBillingEventWorker) Timeout(job *river.Job[models.SendBillingEventArgs]) time.Duration {
	return 10 * time.Second
}

func (w *SendLagoBillingEventWorker) Work(ctx context.Context, job *river.Job[models.SendBillingEventArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Sending billing event", "event", job.Args.Event)

	if err := w.repository.SendEvent(ctx, job.Args.Event); err != nil {
		return err
	}

	logger.DebugContext(ctx, "Billing event sent", "event", job.Args.Event)

	return nil
}
