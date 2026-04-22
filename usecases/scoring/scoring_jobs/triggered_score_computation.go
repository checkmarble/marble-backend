package scoring_jobs

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scoring"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type featureAccessReader interface {
	GetOrganizationFeatureAccess(
		ctx context.Context,
		organizationId uuid.UUID,
		userId *models.UserId,
	) (models.OrganizationFeatureAccess, error)
}

type TriggeredScoreComputationWorker struct {
	river.WorkerDefaults[models.TriggeredScoreComputationArgs]

	executorFactory     executor_factory.ExecutorFactory
	transactionFactory  executor_factory.TransactionFactory
	featureAccessReader featureAccessReader
	rulesetUsecase      scoring.ScoringRulesetsUsecase
	scoreUsecase        scoring.ScoringScoresUsecase
	repository          scoring.ScoringRepository
	offloadedReadWriter repositories.OffloadedReadWriter
}

func NewTriggeredScoreComputationWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	featureAccessReader featureAccessReader,
	rulesetUsecase scoring.ScoringRulesetsUsecase,
	scoreUsecase scoring.ScoringScoresUsecase,
	repository scoring.ScoringRepository,
	offloadedReadWriter repositories.OffloadedReadWriter,
) *TriggeredScoreComputationWorker {
	return &TriggeredScoreComputationWorker{
		executorFactory:     executorFactory,
		transactionFactory:  transactionFactory,
		featureAccessReader: featureAccessReader,
		rulesetUsecase:      rulesetUsecase,
		scoreUsecase:        scoreUsecase,
		repository:          repository,
		offloadedReadWriter: offloadedReadWriter,
	}
}

func (w *TriggeredScoreComputationWorker) Work(ctx context.Context, job *river.Job[models.TriggeredScoreComputationArgs]) error {
	featureAccess, err := w.featureAccessReader.GetOrganizationFeatureAccess(ctx, job.Args.OrgId, nil)
	if err != nil {
		return err
	}
	if !featureAccess.UserScoring.IsAllowed() {
		return nil
	}

	err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		ruleset, err := w.repository.GetScoringRuleset(ctx, tx, job.Args.OrgId, job.Args.RecordType, models.ScoreRulesetCommitted, 0)
		if err != nil {
			// Not having a ruleset here is okay, it means scoring is not configured
			// for that table.
			if errors.Is(err, models.NotFoundError) {
				return nil
			}

			return err
		}

		activeScore, err := w.repository.GetActiveScore(ctx, tx, models.ScoringRecordRef{
			OrgId:      job.Args.OrgId,
			RecordType: job.Args.RecordType,
			RecordId:   job.Args.RecordId,
		})
		if err != nil && !errors.Is(err, models.NotFoundError) {
			return err
		}

		if activeScore.IsOverridden() {
			return nil
		}

		eval, err := w.scoreUsecase.InternalComputeScore(ctx, tx, job.Args.OrgId, ruleset, job.Args.RecordType, job.Args.RecordId)
		if err != nil {
			return err
		}
		if eval == nil {
			return nil
		}

		req := models.InsertScoreRequest{
			OrgId:      job.Args.OrgId,
			RecordType: job.Args.RecordType,
			RecordId:   job.Args.RecordId,
			RiskLevel:  eval.RiskLevel,
			Source:     models.ScoreSourceRuleset,
			RulesetId:  &ruleset.Id,
		}

		if activeScore != nil && eval.RiskLevel < activeScore.RiskLevel {
			if activeScore.CreatedAt.Add(ruleset.Cooldown).After(time.Now()) {
				req.IgnoredByCooldown = true
			}
		}

		score, err := w.repository.InsertScore(ctx, tx, req)
		if err != nil {
			return err
		}

		scoreEvaluations := pure_utils.Map(eval.Evaluation, func(ne ast.NodeEvaluation) *ast.NodeEvaluationDto {
			return utils.Ptr(ast.AdaptNodeEvaluationDto(ne))
		})

		scoreEvaluationsSer, err := dbmodels.SerializeDecisionEvaluationDto(scoreEvaluations)
		if err != nil {
			return err
		}

		if err := w.offloadedReadWriter.OffloadScoreComputation(ctx, ruleset, score, scoreEvaluationsSer); err != nil {
			return errors.Wrap(err, "could not offload score computation")
		}

		return nil
	})

	return err
}
