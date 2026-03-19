package scoring_jobs

import (
	"context"
	"sync"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scoring"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
)

const (
	RULESET_DRY_RUN_PARALLELISM = 15
)

var errDryRunCancelled = errors.New("dry run cancelled")

type RulesetDryRunWorker struct {
	river.WorkerDefaults[models.RulesetDryRunArgs]

	executorFactory            executor_factory.ExecutorFactory
	rulesetUsecase             scoring.ScoringRulesetsUsecase
	scoreUsecase               scoring.ScoringScoresUsecase
	repository                 scoring.ScoringRepository
	ingestedDataReadRepository repositories.IngestedDataReadRepository
}

func NewRulesetDryRunWorker(
	executorFactory executor_factory.ExecutorFactory,
	rulesetUsecase scoring.ScoringRulesetsUsecase,
	scoreUsecase scoring.ScoringScoresUsecase,
	repository scoring.ScoringRepository,
	ingestedDataReadRepository repositories.IngestedDataReadRepository,
) *RulesetDryRunWorker {
	return &RulesetDryRunWorker{
		executorFactory:            executorFactory,
		rulesetUsecase:             rulesetUsecase,
		scoreUsecase:               scoreUsecase,
		repository:                 repository,
		ingestedDataReadRepository: ingestedDataReadRepository,
	}
}

func (w *RulesetDryRunWorker) Work(ctx context.Context, job *river.Job[models.RulesetDryRunArgs]) error {
	if !infra.HasFeatureFlag(infra.FEATURE_USER_SCORING, job.Args.OrgId) {
		return nil
	}

	exec := w.executorFactory.NewExecutor()

	ruleset, err := w.repository.GetScoringRulesetById(ctx, exec, job.Args.OrgId, job.Args.RulesetId)
	if err != nil {
		return err
	}

	dryRun, err := w.repository.GetScoringDryRunById(ctx, exec, job.Args.DryRunId)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			return nil
		}

		return err
	}

	clientDb, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	recordIds, err := w.ingestedDataReadRepository.SampleObjectIds(ctx, clientDb, ruleset.RecordType, dryRun.RecordCount)
	if err != nil {
		return err
	}

	sem := make(chan struct{}, RULESET_DRY_RUN_PARALLELISM)

	var (
		wg           sync.WaitGroup
		lock         = &sync.Mutex{}
		count        = 0
		distribution = make(map[int]int)
	)

	logger := utils.LoggerFromContext(ctx)

	for _, recordId := range recordIds {
		wg.Add(1)

		sem <- struct{}{}

		err := func() error {
			lock.Lock()
			defer lock.Unlock()

			if count >= RULESET_DRY_RUN_PARALLELISM {
				count = 0

				dryRun, err = w.repository.GetScoringDryRunById(ctx, exec, job.Args.DryRunId)
				if err != nil {
					return err
				}

				newDryRun, err := w.repository.SetRulesetDryRunStatus(ctx, exec, dryRun, models.DryRunRunning, distribution)
				if err != nil {
					return err
				}
				if newDryRun == nil { // Support for cancellation
					return errDryRunCancelled
				}
			}

			return nil
		}()
		if err != nil {
			if errors.Is(err, errDryRunCancelled) {
				return nil
			}

			return err
		}

		go func(recordId string) {
			defer wg.Done()
			defer func() { <-sem }()

			eval, err := w.scoreUsecase.InternalComputeScore(ctx, exec, job.Args.OrgId, ruleset, ruleset.RecordType, recordId)
			if err != nil {
				logger.ErrorContext(ctx, "could not compute score",
					"org_id", job.Args.OrgId,
					"ruleset_id", job.Args.RulesetId,
					"dry_run_id", job.Args.DryRunId,
					"error", err.Error())
				return
			}

			lock.Lock()
			defer lock.Unlock()

			if _, ok := distribution[eval.RiskLevel]; !ok {
				distribution[eval.RiskLevel] = 1
			} else {
				distribution[eval.RiskLevel] += 1
			}

			count += 1
		}(recordId)
	}

	wg.Wait()

	if _, err := w.repository.SetRulesetDryRunStatus(ctx, exec, dryRun, models.DryRunCompleted, distribution); err != nil {
		return err
	}

	return nil
}
