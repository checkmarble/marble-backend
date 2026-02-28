package scoring_jobs

import (
	"context"
	"errors"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scoring"
	"github.com/riverqueue/river"
)

type TriggeredScoreComputationWorker struct {
	river.WorkerDefaults[models.TriggeredScoreComputationArgs]

	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	rulesetUsecase     scoring.ScoringRulesetsUsecase
	scoreUsecase       scoring.ScoringScoresUsecase
	repository         scoring.ScoringRepository
}

func NewTriggeredScoreComputationWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	rulesetUsecase scoring.ScoringRulesetsUsecase,
	scoreUsecase scoring.ScoringScoresUsecase,
	repository scoring.ScoringRepository,
) *TriggeredScoreComputationWorker {
	return &TriggeredScoreComputationWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		rulesetUsecase:     rulesetUsecase,
		scoreUsecase:       scoreUsecase,
		repository:         repository,
	}
}

func (w *TriggeredScoreComputationWorker) Work(ctx context.Context, job *river.Job[models.TriggeredScoreComputationArgs]) error {
	if !infra.HasFeatureFlag(infra.FEATURE_USER_SCORING, job.Args.OrgId) {
		return nil
	}

	err := w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		ruleset, err := w.repository.GetScoringRuleset(ctx, tx, job.Args.OrgId, job.Args.EntityType, models.ScoreRulesetCommitted)
		if err != nil {
			// Not having a ruleset here is okay, it means scoring is not configured
			// for that table.
			if errors.Is(err, models.NotFoundError) {
				return nil
			}

			return err
		}

		activeScore, err := w.repository.GetActiveScore(ctx, tx, models.ScoringEntityRef{
			OrgId:      job.Args.OrgId,
			EntityType: job.Args.EntityType,
			EntityId:   job.Args.EntityId,
		})
		if err != nil && !errors.Is(err, models.NotFoundError) {
			return err
		}

		if activeScore.IsOverriden() {
			return nil
		}

		eval, err := w.scoreUsecase.InternalComputeScore(ctx, tx, job.Args.OrgId, ruleset, job.Args.EntityType, job.Args.EntityId)
		if err != nil {
			return err
		}
		if eval == nil {
			return nil
		}

		req := models.InsertScoreRequest{
			OrgId:      job.Args.OrgId,
			EntityType: job.Args.EntityType,
			EntityId:   job.Args.EntityId,
			Score:      eval.Score,
			Source:     models.ScoreSourceRuleset,
		}

		_, err = w.repository.InsertScore(ctx, tx, req)

		return err
	})

	return err
}
