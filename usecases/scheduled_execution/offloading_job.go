package scheduled_execution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"gocloud.dev/blob"
)

const (
	// TODO: find proper interval and batch size, and make them configurable
	OFFLOADING_JOB_INTERVAL          = 20 * time.Second
	OFFLOADING_DECISIONS_BATCH_SIZE  = 1_000
	OFFLOADING_SAVE_POINT_BATCH_SIZE = 100
)

type offloadingRepository interface {
	GetOffloadingWatermark(ctx context.Context, exec repositories.Executor, orgId, table string) (*models.OffloadingWatermark, error)
	SaveOffloadingWatermark(ctx context.Context, tx repositories.Transaction,
		orgId, table, watermarkId string, watermarkTime time.Time) error

	GetOffloadableDecisionRules(ctx context.Context, exec repositories.Executor,
		req models.OffloadDecisionRuleRequest) ([]models.OffloadableDecisionRule, error)
	RemoveDecisionRulePayload(ctx context.Context, tx repositories.Transaction, ids []string) error
}

func NewOffloadingPeriodicJob(orgId string) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(OFFLOADING_JOB_INTERVAL),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.OffloadingArgs{
					OrgId: orgId,
				}, &river.InsertOpts{
					Queue: orgId,
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: OFFLOADING_JOB_INTERVAL,
						ByState: []rivertype.JobState{
							rivertype.JobStateAvailable,
							rivertype.JobStatePending,
							rivertype.JobStateScheduled,
							rivertype.JobStateRunning,
						},
					},
				}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

type OffloadingWorker struct {
	river.WorkerDefaults[models.OffloadingArgs]

	executorFactory     executor_factory.ExecutorFactory
	transactionFactory  executor_factory.TransactionFactory
	repository          offloadingRepository
	blobRepository      repositories.BlobRepository
	offloadingBucketUrl string
}

func NewOffloadingWorker(executorFactory executor_factory.ExecutorFactory, transactionFactory executor_factory.TransactionFactory,
	repository offloadingRepository, blobRepository repositories.BlobRepository, offloadingBucketUrl string,
) *OffloadingWorker {
	return &OffloadingWorker{
		executorFactory:     executorFactory,
		transactionFactory:  transactionFactory,
		repository:          repository,
		blobRepository:      blobRepository,
		offloadingBucketUrl: offloadingBucketUrl,
	}
}

func (w OffloadingWorker) Work(ctx context.Context, job *river.Job[models.OffloadingArgs]) error {
	logger := utils.LoggerFromContext(ctx)

	exec := w.executorFactory.NewExecutor()

	wm, err := w.repository.GetOffloadingWatermark(ctx, exec, job.Args.OrgId, "decision_rules")
	if err != nil {
		return err
	}

	req := models.OffloadDecisionRuleRequest{
		OrgId: job.Args.OrgId,
		// TODO: make offloading threshold configurable?
		DeleteBefore: time.Now().Add(-24 * 30 * time.Hour),
		BatchSize:    OFFLOADING_DECISIONS_BATCH_SIZE,
		Watermark:    wm,
	}

	rules, err := w.repository.GetOffloadableDecisionRules(ctx, w.executorFactory.NewExecutor(), req)
	if err != nil {
		return err
	}

	// We are up to date.
	if len(rules) == 0 {
		return nil
	}

	if wm == nil {
		logger.Debug(fmt.Sprintf("found %d decisions rules to offload", len(rules)), "org_id", job.Args.OrgId)
	} else {
		logger.Debug(fmt.Sprintf("found %d decisions rules to offload", len(rules)), "org_id",
			job.Args.OrgId, "watermark_id", wm.WatermarkId, "watermark_time", wm.WatermarkTime)
	}

	offloadedIds := make([]string, OFFLOADING_SAVE_POINT_BATCH_SIZE)

	for idx, rule := range rules {
		key := fmt.Sprintf("offloading/decision_rules/%s/%d/%d/%s/%s", req.OrgId,
			rule.CreatedAt.Year(), rule.CreatedAt.Month(), rule.DecisionId, rule.Rule.Id)

		opts := blob.WriterOptions{Metadata: map[string]string{
			"Custom-Date": rule.CreatedAt.Format(time.RFC3339),
		}}

		wr, err := w.blobRepository.OpenStreamWithOptions(ctx, w.offloadingBucketUrl, key, &opts)
		if err != nil {
			return err
		}
		defer wr.Close()

		enc := json.NewEncoder(wr)

		if err := enc.Encode(rule.Evaluation); err != nil {
			return err
		}

		if err := wr.Close(); err != nil {
			return err
		}

		offloadedIds[idx%OFFLOADING_SAVE_POINT_BATCH_SIZE] = rule.Id

		// If we get here, we have a full batch, so we can send the whole slice to be persisted.
		// See below for the other case.
		if (idx+1)%OFFLOADING_SAVE_POINT_BATCH_SIZE == 0 {
			err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
				fmt.Println("SAVING", rule.DecisionId)

				if err := w.repository.RemoveDecisionRulePayload(ctx, tx, offloadedIds); err != nil {
					return err
				}
				if err := w.repository.SaveOffloadingWatermark(ctx, tx, job.Args.OrgId,
					"decision_rules", rule.DecisionId, rule.CreatedAt); err != nil {
					return err
				}

				logger.Debug(fmt.Sprintf("successfully offloaded %d decision rules", idx+1), "org_id", job.Args.OrgId)

				logger.Debug(fmt.Sprintf("successfully offloaded %d decision rules", idx), "org_id", job.Args.OrgId)

				return nil
			})
			if err != nil {
				return err
			}
		}
	}

	// If we did not get a multiple of of save point batch size, we still have len(rules)%batch_size
	// items to persist.
	if len(rules)%OFFLOADING_SAVE_POINT_BATCH_SIZE > 0 {
		lastDecisionRule := rules[len(rules)-1]
		remainingItems := offloadedIds[:len(rules)%OFFLOADING_SAVE_POINT_BATCH_SIZE]

		return w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
			if err := w.repository.RemoveDecisionRulePayload(ctx, tx, remainingItems); err != nil {
				return err
			}
			if err := w.repository.SaveOffloadingWatermark(ctx, tx, job.Args.OrgId, "decision_rules",
				lastDecisionRule.DecisionId, lastDecisionRule.CreatedAt); err != nil {
				return err
			}

			return nil
		})
	}

	return nil
}
