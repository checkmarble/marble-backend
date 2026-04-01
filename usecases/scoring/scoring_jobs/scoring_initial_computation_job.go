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

const (
	INITIAL_COMPUTATION_PARALLELISM = 5
	INITIAL_COMPUTATION_BATCH_SIZE  = 10_000
)

func NewInitialComputationJob(orgId uuid.UUID, interval time.Duration) *river.PeriodicJob {
	return worker_jobs.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.ScoringInitialComputationArgs{
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

type InitialComputationWorker struct {
	river.WorkerDefaults[models.ScoringInitialComputationArgs]

	executorFactory executor_factory.ExecutorFactory
	scoreUsecase    scoring.ScoringScoresUsecase
	repository      scoring.ScoringRepository
}

func NewInitialComputationWorker(
	executorFactory executor_factory.ExecutorFactory,
	scoreUsecase scoring.ScoringScoresUsecase,
	repository scoring.ScoringRepository,
) *InitialComputationWorker {
	return &InitialComputationWorker{
		executorFactory: executorFactory,
		scoreUsecase:    scoreUsecase,
		repository:      repository,
	}
}

func (w *InitialComputationWorker) Timeout(job *river.Job[models.ScoringInitialComputationArgs]) time.Duration {
	return time.Hour
}

func (w *InitialComputationWorker) Work(ctx context.Context, job *river.Job[models.ScoringInitialComputationArgs]) error {
	if !infra.HasFeatureFlag(infra.FEATURE_USER_SCORING, job.Args.OrgId) {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(w.Timeout(job).Seconds()*0.90)*time.Second)
	defer cancel()

	logger := utils.LoggerFromContext(ctx).With("org_id", job.Args.OrgId.String())
	exec := w.executorFactory.NewExecutor()

	rulesets, err := w.repository.ListScoringRulesets(ctx, exec, job.Args.OrgId)
	if err != nil {
		return err
	}

	var rulesetWg sync.WaitGroup

	for _, r := range rulesets {
		ruleset, err := w.repository.GetScoringRuleset(ctx, exec, job.Args.OrgId, r.RecordType, models.ScoreRulesetCommitted, 0)
		if err != nil {
			if errors.Is(err, models.NotFoundError) {
				continue
			}

			return err
		}

		logger := logger.With("record_type", ruleset.RecordType)

		rulesetWg.Go(func() {
			exec := w.executorFactory.NewExecutor()
			computed := 0

			defer func() {
				logger.InfoContext(ctx, "computed initial scores",
					"scores", computed)
			}()

			sem := make(chan struct{}, INITIAL_COMPUTATION_PARALLELISM)

			opts := models.RefreshScoreOptions{RefreshOlderThan: 0, RefreshInBackground: false}

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				unscoredIds, err := w.repository.GetUnscoredBatch(ctx, exec, job.Args.OrgId, ruleset.RecordType, INITIAL_COMPUTATION_BATCH_SIZE)
				if err != nil {
					logger.ErrorContext(ctx, "could not retrieve unscored records",
						"error", err.Error())
					return
				}

				if len(unscoredIds) == 0 {
					return
				}

				var batchWg sync.WaitGroup

			batchLoop:
				for _, recordId := range unscoredIds {
					select {
					case sem <- struct{}{}:
					case <-ctx.Done():
						break batchLoop
					}

					computed += 1

					batchWg.Add(1)

					go func(recordId string) {
						defer batchWg.Done()
						defer func() { <-sem }()

						record := models.ScoringRecordRef{
							OrgId:      job.Args.OrgId,
							RecordType: ruleset.RecordType,
							RecordId:   recordId,
						}

						if _, _, err := w.scoreUsecase.GetActiveScore(ctx, record, false, opts); err != nil {
							if ctx.Err() == nil {
								logger.ErrorContext(ctx, "could not compute initial score",
									"error", err.Error())
							}
						}
					}(recordId)
				}

				batchWg.Wait()
			}
		})
	}

	rulesetWg.Wait()

	return nil
}
