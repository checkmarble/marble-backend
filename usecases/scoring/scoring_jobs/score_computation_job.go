package scoring_jobs

import (
	"context"
	"sync"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scoring"
	"github.com/checkmarble/marble-backend/usecases/worker_jobs"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const RESCORING_BATCH_SIZE = 1000

func NewScoreComputationJob(orgId uuid.UUID, interval time.Duration) *river.PeriodicJob {
	return worker_jobs.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.ScoreComputationArgs{
					OrgId: orgId,
				}, &river.InsertOpts{
					Queue: orgId.String(),
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: interval,
					},
				}
		},
	)
}

type ScoreComputationWorker struct {
	river.WorkerDefaults[models.ScoreComputationArgs]

	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	scoreUsecase       scoring.ScoringScoresUsecase
	repository         scoring.ScoringRepository
}

func NewScoreComputationWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	scoreUsecase scoring.ScoringScoresUsecase,
	repository scoring.ScoringRepository,
) *ScoreComputationWorker {
	return &ScoreComputationWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		scoreUsecase:       scoreUsecase,
		repository:         repository,
	}
}

func (w *ScoreComputationWorker) Timeout(job *river.Job[models.ScoreComputationArgs]) time.Duration {
	return time.Hour
}

func (w *ScoreComputationWorker) Work(ctx context.Context, job *river.Job[models.ScoreComputationArgs]) error {
	if !infra.HasFeatureFlag(infra.FEATURE_USER_SCORING, job.Args.OrgId) {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(w.Timeout(job).Seconds()*0.90)*time.Second)
	defer cancel()

	logger := utils.LoggerFromContext(ctx)
	exec := w.executorFactory.NewExecutor()

	rulesets, err := w.repository.ListScoringRulesets(ctx, exec, job.Args.OrgId)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	for _, r := range rulesets {
		ruleset, err := w.repository.GetScoringRuleset(ctx, exec, job.Args.OrgId, r.RecordType, models.ScoreRulesetCommitted, 0)
		if err != nil {
			if errors.Is(err, models.NotFoundError) {
				continue
			}

			return err
		}

		watermark := time.Now().Add(-ruleset.ScoringInterval)

		wg.Go(func() {
			exec := w.executorFactory.NewExecutor()
			recomputed := 0

			defer func() {
				logger.InfoContext(ctx, "recomputed scores",
					"org_id", job.Args.OrgId.String(),
					"record_type", ruleset.RecordType,
					"scores", recomputed)
			}()

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				staleRecordIds, err := w.repository.GetStaleScoreBatch(
					ctx, exec,
					job.Args.OrgId,
					ruleset.RecordType,
					watermark,
					RESCORING_BATCH_SIZE)
				if err != nil {
					logger.ErrorContext(ctx, "could not retrieve stale scores",
						"error", err.Error())
					return
				}

				if len(staleRecordIds) == 0 {
					return
				}

				opts := models.RefreshScoreOptions{RefreshOlderThan: ruleset.ScoringInterval, RefreshInBackground: false}

				for _, recordId := range staleRecordIds {
					record := models.ScoringRecordRef{
						OrgId:      job.Args.OrgId,
						RecordType: ruleset.RecordType,
						RecordId:   recordId,
					}

					// TODO: what behavior do we want here? Aborting will get the
					// recomputation stuck. Skipping will ultimately make it so the full
					// batch might be erroring.
					if _, _, err := w.scoreUsecase.GetActiveScore(ctx, record, false, opts); err != nil {
						logger.ErrorContext(ctx, "could not recompute score",
							"error", err.Error())
						return
					}

					recomputed += 1
				}
			}
		})
	}

	wg.Wait()

	return nil
}
