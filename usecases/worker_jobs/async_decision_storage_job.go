package worker_jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"
)

type asyncDecisionStorageUsecase interface {
	StoreDecision(ctx context.Context, logger *slog.Logger, tx repositories.Transaction, item models.DecisionBundle) (models.DecisionWithRuleExecutions, error)
}

type AsyncDecisionStorageWorker struct {
	river.WorkerDefaults[models.AsyncDecisionStorageArgs]

	transactionFactory executor_factory.TransactionFactory
	redisClient        *repositories.RedisClient
	decisionUsecase    asyncDecisionStorageUsecase
}

func NewAsyncDecisionStorageWorker(
	transactionFactory executor_factory.TransactionFactory,
	redisClient *repositories.RedisClient,
	decisionUsecase asyncDecisionStorageUsecase,
) AsyncDecisionStorageWorker {
	return AsyncDecisionStorageWorker{
		transactionFactory: transactionFactory,
		redisClient:        redisClient,
		decisionUsecase:    decisionUsecase,
	}
}

func (w AsyncDecisionStorageWorker) Work(ctx context.Context, job *river.Job[models.AsyncDecisionStorageArgs]) error {
	storageStart := time.Now()

	logger := utils.LoggerFromContext(ctx).With(
		"org_id", job.Args.Bundle.Decision.OrganizationId,
		"decision", job.Args.Bundle.Decision.DecisionId)

	err := w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		_, err := w.decisionUsecase.StoreDecision(
			ctx,
			logger,
			tx,
			job.Args.Bundle,
		)

		return err
	})

	switch {
	case err == nil || repositories.IsUniqueViolationErrorOf(err, "decisions_pkey"):
		w.deleteTransientDecision(ctx, job, logger)

		switch err {
		case nil:
			logger.InfoContext(ctx, "asynchronously stored decision",
				"latency", time.Since(storageStart))

		default:
			logger.InfoContext(ctx, "skip duplicate async decision storage because it is already persisted",
				"latency", time.Since(storageStart))
		}

	default:
		logger.ErrorContext(ctx, "could not asynchronously store decision",
			"err", err.Error())

		return err
	}

	return nil
}

func (w AsyncDecisionStorageWorker) deleteTransientDecision(ctx context.Context, job *river.Job[models.AsyncDecisionStorageArgs], logger *slog.Logger) {
	if w.redisClient != nil {
		redisExec := w.redisClient.NewExecutor(job.Args.Bundle.Decision.OrganizationId)
		key := redisExec.Key("decision", job.Args.Bundle.Decision.DecisionId.String())

		err := redisExec.Exec(func(c *redis.Client) error {
			return c.Del(ctx, key).Err()
		})

		if err != nil {
			logger.WarnContext(ctx, "could not delete transient decision from the cache",
				"err", err.Error())
		}
	}
}
