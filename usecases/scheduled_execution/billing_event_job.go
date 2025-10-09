package scheduled_execution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/billing"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/riverqueue/river"
)

const BATCH_SIZE = 10000

var SubscriptionsCache = expirable.NewLRU[string, string](100, nil, 0)

type MarbleRepository interface {
	SaveWatermark(
		ctx context.Context,
		exec repositories.Executor,
		orgId *string,
		watermarkType models.WatermarkType,
		watermarkId *string,
		watermarkTime time.Time,
		params json.RawMessage,
	) error
	GetWatermark(
		ctx context.Context,
		exec repositories.Executor,
		orgId *string,
		watermarkType models.WatermarkType,
	) (*models.Watermark, error)
}

type BillingUsecase interface {
	SendEventsAsync(ctx context.Context, tx repositories.Transaction, events []models.BillingEvent) error
	GetSubscriptionsForEvent(
		ctx context.Context,
		orgId string,
		code billing.BillableMetric,
	) ([]models.Subscription, error)
}

type BillingEventPeriodicJob struct {
	river.WorkerDefaults[models.BillingEventPeriodicJobArgs]

	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	marbleRepository MarbleRepository
	billingUsercase  BillingUsecase
}

func NewBillingEventPeriodicJob(
	repository MarbleRepository,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	billingUsercase BillingUsecase,
) BillingEventPeriodicJob {
	return BillingEventPeriodicJob{
		marbleRepository:   repository,
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		billingUsercase:    billingUsercase,
	}
}

func (w BillingEventPeriodicJob) Timeout(job *river.Job[models.BillingEventPeriodicJobArgs]) time.Duration {
	return time.Minute
}

func (w BillingEventPeriodicJob) Work(ctx context.Context, job *river.Job[models.BillingEventPeriodicJobArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Starting billing event periodic job")

	exec := w.executorFactory.NewExecutor()

	now := time.Now()
	from := now.Add(-1 * time.Hour) // get from watermark
	lastTimestamp := now

	offset := 0
	for {
		decisions, err := w.marbleRepository.GetDecisions(ctx, from, now, offset, BATCH_SIZE)
		if err != nil {
			return err
		}

		if len(decisions) == 0 {
			break
		}

		logger.DebugContext(ctx, "Processing batch of decisions", "offset", offset, "nb_decisions", len(decisions))

		events := make([]models.BillingEvent, len(decisions))
		for i, decision := range decisions {
			externalSubscriptionId, err := w.getSubscriptionsForEvent(ctx,
				decision.OrganizationId, billing.DECISION)
			if err != nil {
				return err
			}
			events[i] = models.BillingEvent{
				TransactionId:          decision.Id,
				Code:                   billing.DECISION.String(),
				Timestamp:              decision.CreatedAt,
				ExternalSubscriptionId: externalSubscriptionId,
			}
		}

		err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
			return w.billingUsercase.SendEventsAsync(ctx, tx, events)
		})
		if err != nil {
			return err
		}

		lastTimestamp = decisions[len(decisions)-1].CreatedAt

		w.marbleRepository.SaveWatermark(
			ctx,
			exec,
			nil,
			models.WatermarkTypeBillingDecisions,
			nil,
			lastTimestamp,
			nil,
		)

		if len(decisions) < BATCH_SIZE {
			break
		}

		offset += len(decisions)
	}

	return nil
}

func (w BillingEventPeriodicJob) getSubscriptionsForEvent(ctx context.Context, orgId string,
	code billing.BillableMetric,
) (string, error) {
	// With cache system
	cacheKey := fmt.Sprintf("%s-%s", orgId, code.String())
	if cached, ok := SubscriptionsCache.Get(cacheKey); ok {
		return cached, nil
	}

	subscriptions, err := w.billingUsercase.GetSubscriptionsForEvent(ctx, orgId, code)
	if err != nil {
		return "", err
	}

	SubscriptionsCache.Add(cacheKey, subscriptions[0].ExternalId)
	return subscriptions[0].ExternalId, nil
}
