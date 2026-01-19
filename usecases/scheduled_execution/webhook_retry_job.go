package scheduled_execution

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const (
	WEBHOOK_RETRY_INTERVAL = 10 * time.Minute
	WEBHOOK_RETRY_TIMEOUT  = 5 * time.Minute
)

func NewWebhookRetryPeriodicJob(orgId uuid.UUID) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(WEBHOOK_RETRY_INTERVAL),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.WebhookRetryArgs{OrgId: orgId},
				&river.InsertOpts{
					Queue: orgId.String(),
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: WEBHOOK_RETRY_INTERVAL,
					},
				}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

type WebhookEventsRetryUsecase interface {
	RetrySendWebhookEventsForOrg(ctx context.Context, orgId uuid.UUID) error
}

type WebhookRetryWorker struct {
	river.WorkerDefaults[models.WebhookRetryArgs]

	webhookEventsUsecase WebhookEventsRetryUsecase
}

func NewWebhookRetryWorker(
	webhookEventsUsecase WebhookEventsRetryUsecase,
) *WebhookRetryWorker {
	return &WebhookRetryWorker{
		webhookEventsUsecase: webhookEventsUsecase,
	}
}

func (w *WebhookRetryWorker) Timeout(job *river.Job[models.WebhookRetryArgs]) time.Duration {
	return WEBHOOK_RETRY_TIMEOUT
}

func (w *WebhookRetryWorker) Work(ctx context.Context, job *river.Job[models.WebhookRetryArgs]) error {
	return w.webhookEventsUsecase.RetrySendWebhookEventsForOrg(ctx, job.Args.OrgId)
}
