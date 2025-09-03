package scheduled_execution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gocloud.dev/blob"
	"golang.org/x/time/rate"
)

type offloadingRepository interface {
	GetOffloadedDecisionRuleKey(orgId, decisionId, ruleId, outcome string, createdAt time.Time) string

	GetWatermark(ctx context.Context, exec repositories.Executor, orgId *string,
		watermarkType models.WatermarkType) (*models.Watermark, error)
	SaveWatermark(ctx context.Context, exec repositories.Executor,
		orgId *string, watermarkType models.WatermarkType, watermarkId *string, watermarkTime time.Time, params json.RawMessage) error

	GetOffloadableDecisionRules(
		ctx context.Context,
		tx repositories.Transaction,
		req models.OffloadDecisionRuleRequest,
	) (models.ChannelOfModels[models.OffloadableDecisionRule], error)
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
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	logger := utils.LoggerFromContext(ctx)
	exec := w.executorFactory.NewExecutor()

	// unicity of jobs per queue ignoreing completed or discarded jobs is unpractical, see the documentation of UniqueOpts.
	if job.CreatedAt.Before(time.Now().Add(-w.config.JobInterval)) {
		logger.DebugContext(ctx, "skipping offloading job instance because it was created too long ago. A new one should have been created.", "job_created_at", job.CreatedAt)
		return nil
	}

	grace := time.Duration(w.config.JobInterval.Seconds()*0.90) * time.Second
	if grace.Minutes() > 3 {
		grace = 3 * time.Minute
	}

	timeout := time.After(w.config.JobInterval - grace)

	// The first iteration should use a large inequality, because a previous iteration of the offloading job may not have offloaded
	// every rule of the watermarked decision. It will then be set to false for the next iterations to avoid an infinite loop once the end of the decisions is reached.
	useLargeInequality := true

	for {
		ctx, span := tracer.Start(ctx, "offloading_batch")
		defer span.End()

		wm, err := w.repository.GetWatermark(ctx, exec, &job.Args.OrgId, models.WatermarkTypeDecisionRules)
		if err != nil {
			return err
		}

		req := models.OffloadDecisionRuleRequest{
			OrgId:           job.Args.OrgId,
			DeleteBefore:    time.Now().Add(-w.config.OffloadBefore),
			BatchSize:       w.config.BatchSize,
			Watermark:       wm,
			LargeInequality: useLargeInequality,
		}

		outerTx, err := exec.Begin(ctx)
		if err != nil {
			return err
		}
		defer outerTx.Rollback(ctx) //nolint:errcheck

		// The transaction is only so we can "set local" to disable hash joins in the repo call below - writes should still be made outside of the transaction,
		// as we specificially don't want to wait for the end of the transaction to write the save points.
		channels, err := w.repository.GetOffloadableDecisionRules(ctx, outerTx, req)
		if err != nil {
			return err
		}
		// The next iterations should use a strict inequality to avoid an infinite loop of decisions.
		useLargeInequality = false

		stopChannelAndReturn := func(err error) error {
			channels.CloseAndWaitUntilDone()
			return err
		}

		span.AddEvent("got_offloadable")

		// Slice of pointers to not have to keep track of which decision were skipped or not, the
		// query ignores NULL IDs.
		offloadedIds := make([]*string, w.config.SavepointEvery)
		idx := 0

		var lastOfBatch *models.OffloadableDecisionRule
		for item := range channels.Models {
			select {
			case <-timeout:
				return stopChannelAndReturn(nil)
			case <-ctx.Done():
				return stopChannelAndReturn(ctx.Err())
			default:
				if err := w.limiter.Wait(ctx); err != nil {
					return stopChannelAndReturn(err)
				}
			}

			if item.Error != nil {
				return stopChannelAndReturn(item.Error)
			}

			rule := item.Model
			offloadedIds[idx%w.config.SavepointEvery] = nil

			if rule.RuleEvaluation != nil && rule.RuleOutcome != nil {
				key := w.repository.GetOffloadedDecisionRuleKey(req.OrgId,
					rule.DecisionId, *rule.RuleId, *rule.RuleOutcome, rule.CreatedAt)

				opts := blob.WriterOptions{}

				// Only if we work on GCS, retrieve the typed writer and set the Custom-Time fixed metadata to the rule creation time.
				// We do not (currently) set the date on other blob storage platforms (but maybe we should?)
				opts.BeforeWrite = func(asFunc func(any) bool) error {
					var gcsWriter *storage.Writer

					if asFunc(&gcsWriter) {
						gcsWriter.CustomTime = rule.CreatedAt
					}

					return nil
				}

				wr, err := w.blobRepository.OpenStreamWithOptions(ctx, w.config.BucketUrl, key, &opts)
				if err != nil {
					return stopChannelAndReturn(err)
				}
				defer wr.Close()

				enc := json.NewEncoder(wr)

				if err := enc.Encode(rule.RuleEvaluation); err != nil {
					if err := wr.Close(); err != nil {
						logger.Error(fmt.Sprintf("could not close writer after write failure: %s", err.Error()))
					}
					return stopChannelAndReturn(err)
				}

				if err := wr.Close(); err != nil {
					return stopChannelAndReturn(err)
				}

				id := *rule.RuleExecutionId
				offloadedIds[idx%w.config.SavepointEvery] = &id
			}

			idx += 1

			// If we get here, we have a full batch, so we can send the whole slice to be persisted.
			// See below for the other case.
			if idx%w.config.SavepointEvery == 0 {
				err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
					if err := w.repository.RemoveDecisionRulePayload(ctx, tx, offloadedIds); err != nil {
						return err
					}
					if err := w.repository.SaveWatermark(
						ctx,
						tx,
						&job.Args.OrgId,
						models.WatermarkTypeDecisionRules,
						&rule.DecisionId,
						rule.CreatedAt,
						nil,
					); err != nil {
						return err
					}

					span.AddEvent("savepoint", trace.WithAttributes(attribute.Int("decision_rules", idx)))
					logger.DebugContext(ctx, fmt.Sprintf("offloading save point after %d decision rules", idx), "org_id", job.Args.OrgId)

					return nil
				})
				if err != nil {
					return stopChannelAndReturn(err)
				}
			}

			lastOfBatch = &rule
		}
		_ = stopChannelAndReturn(nil)
		if err := outerTx.Rollback(ctx); err != nil {
			return err
		}

		if idx == 0 {
			return nil
		}

		// If we did not get a multiple of save point batch size, we still have len(rules)%batch_size
		// items to persist.
		if idx%w.config.SavepointEvery > 0 {
			remainingItems := offloadedIds[:idx%w.config.SavepointEvery]

			err := w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
				if err := w.repository.RemoveDecisionRulePayload(ctx, tx, remainingItems); err != nil {
					return err
				}
				if err := w.repository.SaveWatermark(
					ctx,
					tx,
					&job.Args.OrgId,
					models.WatermarkTypeDecisionRules,
					&lastOfBatch.DecisionId,
					lastOfBatch.CreatedAt,
					nil); err != nil {
					return err
				}

				span.AddEvent("savepoint", trace.WithAttributes(attribute.Int("decision_rules", idx)))
				logger.DebugContext(ctx, fmt.Sprintf("offloading last save point after %d decision rules", idx+1), "org_id", job.Args.OrgId)

				return nil
			})
			if err != nil {
				return err
			}
		}

		if wm == nil {
			span.AddEvent("finished", trace.WithAttributes(attribute.Int("decision_rules", idx)))
			logger.DebugContext(ctx, fmt.Sprintf("offloaded batch of %d decisions rules", idx), "org_id", job.Args.OrgId)
		} else {
			span.AddEvent("finished", trace.WithAttributes(
				attribute.Int("decision_rules", idx),
				attribute.String("watermark", wm.WatermarkTime.String())))
			logger.DebugContext(ctx, fmt.Sprintf("offloaded batch of %d decisions rules", idx),
				"org_id", job.Args.OrgId,
				"old_watermark_time", wm.WatermarkTime,
				"old_watermark_id", wm.WatermarkId,
				"new_watermark_time", lastOfBatch.CreatedAt.String(),
				"new_watermark_id", lastOfBatch.RuleExecutionId,
			)
		}

		span.End()
	}
}
