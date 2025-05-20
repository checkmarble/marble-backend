package scheduled_execution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
	"gocloud.dev/blob"
	"golang.org/x/time/rate"
)

type offloadingRepository interface {
	GetOffloadedDecisionRuleKey(orgId, decisionId, ruleId string, createdAt time.Time) string

	GetOffloadingWatermark(ctx context.Context, exec repositories.Executor, orgId, table string) (*models.OffloadingWatermark, error)
	SaveOffloadingWatermark(ctx context.Context, tx repositories.Transaction,
		orgId, table, watermarkId string, watermarkTime time.Time) error

	GetOffloadableDecisionRules(ctx context.Context, exec repositories.Executor,
		req models.OffloadDecisionRuleRequest) (<-chan repositories.ModelResult[models.OffloadableDecisionRule], error)
	RemoveDecisionRulePayload(ctx context.Context, tx repositories.Transaction, ids []*string) error
}

func NewOffloadingPeriodicJob(orgId string, interval time.Duration) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.OffloadingArgs{
					OrgId: orgId,
				}, &river.InsertOpts{
					Queue: orgId,
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: interval,
					},
				}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

type OffloadingWorker struct {
	river.WorkerDefaults[models.OffloadingArgs]

	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	repository         offloadingRepository
	blobRepository     repositories.BlobRepository

	limiter *rate.Limiter
	config  infra.OffloadingConfig
}

func (w *OffloadingWorker) Timeout(job *river.Job[models.OffloadingArgs]) time.Duration {
	// A timeout of JobInterval is okay because we set a custom timeout that will be slightly lower.
	return w.config.JobInterval
}

func NewOffloadingWorker(executorFactory executor_factory.ExecutorFactory, transactionFactory executor_factory.TransactionFactory,
	repository offloadingRepository, blobRepository repositories.BlobRepository,
	offloadingBucketUrl string, offloadConfig infra.OffloadingConfig,
) *OffloadingWorker {
	return &OffloadingWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repository:         repository,
		blobRepository:     blobRepository,
		limiter:            rate.NewLimiter(rate.Limit(offloadConfig.WritesPerSecond), 1),
		config:             offloadConfig,
	}
}

func (w OffloadingWorker) Work(ctx context.Context, job *river.Job[models.OffloadingArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	exec := w.executorFactory.NewExecutor()

	grace := time.Duration(w.config.JobInterval.Seconds()*0.90) * time.Second
	if grace.Minutes() > 3 {
		grace = 3 * time.Minute
	}

	timeout := time.After(w.config.JobInterval - grace)

	for {
		wm, err := w.repository.GetOffloadingWatermark(ctx, exec, job.Args.OrgId, "decision_rules")
		if err != nil {
			return err
		}

		req := models.OffloadDecisionRuleRequest{
			OrgId:        job.Args.OrgId,
			DeleteBefore: time.Now().Add(-w.config.OffloadBefore),
			BatchSize:    w.config.BatchSize,
			Watermark:    wm,
		}

		rules, err := w.repository.GetOffloadableDecisionRules(ctx, exec, req)
		if err != nil {
			return err
		}

		// Slice of pointers to not have to keep track of which decision were skipped or not, the
		// query ignores NULL IDs.
		offloadedIds := make([]*string, w.config.SavepointEvery)
		idx := 0

		var lastOfBatch *models.OffloadableDecisionRule

		for item := range rules {
			select {
			case <-timeout:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			default:
				if err := w.limiter.Wait(ctx); err != nil {
					return err
				}
			}

			if item.Error != nil {
				return item.Error
			}

			rule := item.Model
			offloadedIds[idx%w.config.SavepointEvery] = nil

			if rule.RuleExecutionId != nil && rule.RuleEvaluation != nil {
				key := w.repository.GetOffloadedDecisionRuleKey(req.OrgId,
					rule.DecisionId, *rule.RuleId, rule.CreatedAt)

				opts := blob.WriterOptions{Metadata: map[string]string{
					"Custom-Date": rule.CreatedAt.Format(time.RFC3339),
				}}

				wr, err := w.blobRepository.OpenStreamWithOptions(ctx, w.config.BucketUrl, key, &opts)
				if err != nil {
					return err
				}
				defer wr.Close()

				enc := json.NewEncoder(wr)

				if err := enc.Encode(rule.RuleEvaluation); err != nil {
					return err
				}

				if err := wr.Close(); err != nil {
					return err
				}

				offloadedIds[idx%w.config.SavepointEvery] = rule.RuleExecutionId
			}

			idx += 1

			// If we get here, we have a full batch, so we can send the whole slice to be persisted.
			// See below for the other case.
			if idx%w.config.SavepointEvery == 0 {
				err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
					if err := w.repository.RemoveDecisionRulePayload(ctx, tx, offloadedIds); err != nil {
						return err
					}
					if err := w.repository.SaveOffloadingWatermark(ctx, tx, job.Args.OrgId,
						"decision_rules", rule.DecisionId, rule.CreatedAt); err != nil {
						return err
					}

					logger.Debug(fmt.Sprintf("offloading save point after %d decision rules", idx), "org_id", job.Args.OrgId)

					return nil
				})
				if err != nil {
					return err
				}
			}

			lastOfBatch = &rule
		}

		if idx == 0 {
			return nil
		}

		// If we did not get a multiple of of save point batch size, we still have len(rules)%batch_size
		// items to persist.
		if idx%w.config.SavepointEvery > 0 {
			remainingItems := offloadedIds[:idx%w.config.SavepointEvery]

			err := w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
				if err := w.repository.RemoveDecisionRulePayload(ctx, tx, remainingItems); err != nil {
					return err
				}
				if err := w.repository.SaveOffloadingWatermark(ctx, tx, job.Args.OrgId, "decision_rules",
					lastOfBatch.DecisionId, lastOfBatch.CreatedAt); err != nil {
					return err
				}

				logger.Debug(fmt.Sprintf("offloading save point after %d decision rules", idx+1), "org_id", job.Args.OrgId)

				return nil
			})

			if err != nil {
				return err
			}
		}

		if wm == nil {
			logger.Debug(fmt.Sprintf("offloaded batch of %d decisions rules", idx), "org_id", job.Args.OrgId)
		} else {
			logger.Debug(fmt.Sprintf("offloaded batch of %d decisions rules", idx), "org_id",
				job.Args.OrgId, "watermark_id", wm.WatermarkId, "watermark_time", wm.WatermarkTime)
		}
	}
}
