package scoring

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type ScoringScoresUsecase struct {
	enforceSecurity     security.EnforceSecurityScoring
	executorFactory     executor_factory.ExecutorFactory
	transactionFactory  executor_factory.TransactionFactory
	repository          ScoringRepository
	dataModelRepository repositories.DataModelRepository
	ingestedDataReader  scoringIngestedDataReader
	taskQueueRepository repositories.TaskQueueRepository
	evaluateAst         ast_eval.EvaluateAstExpression
}

func NewScoringScoresUsecase(
	enforceSecurity security.EnforceSecurityScoring,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repository ScoringRepository,
	dataModelRepository repositories.DataModelRepository,
	ingestedDataReader scoringIngestedDataReader,
	taskQueueRepository repositories.TaskQueueRepository,
	evaluateAst ast_eval.EvaluateAstExpression,
) ScoringScoresUsecase {
	return ScoringScoresUsecase{
		enforceSecurity:     enforceSecurity,
		executorFactory:     executorFactory,
		transactionFactory:  transactionFactory,
		repository:          repository,
		dataModelRepository: dataModelRepository,
		ingestedDataReader:  ingestedDataReader,
		taskQueueRepository: taskQueueRepository,
		evaluateAst:         evaluateAst,
	}
}

func (uc ScoringScoresUsecase) ComputeScore(ctx context.Context, entityType, entityId string) (*models.ScoringEvaluation, error) {
	exec := uc.executorFactory.NewExecutor()
	orgId := uc.enforceSecurity.OrgId()

	ruleset, err := uc.repository.GetScoringRuleset(
		ctx,
		exec,
		orgId,
		entityType,
		models.ScoreRulesetCommitted)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			return nil, errors.Wrap(err, "no committed version of this ruleset")
		}

		return nil, err
	}

	return uc.InternalComputeScore(ctx,
		exec,
		orgId,
		ruleset,
		entityType, entityId)
}

func (uc ScoringScoresUsecase) InternalComputeScore(ctx context.Context, exec repositories.Executor, orgId uuid.UUID,
	ruleset models.ScoringRuleset,
	entityType, entityId string,
) (*models.ScoringEvaluation, error) {
	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, orgId, false, false)
	if err != nil {
		return nil, err
	}

	object, err := uc.getPayloadObject(ctx, orgId, dataModel, entityType, entityId)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			return nil, nil
		}
		return nil, err
	}

	env := uc.evaluateAst.AstEvaluationEnvironmentFactory(ast_eval.EvaluationEnvironmentFactoryParams{
		OrganizationId: orgId,
		DataModel:      dataModel,
		ClientObject:   object,
	})

	eval, err := uc.executeRules(ctx, env, ruleset)
	if err != nil {
		return nil, err
	}

	eval.Score = uc.internalScoreToScore(ruleset, eval)

	return &eval, nil
}

func (uc ScoringScoresUsecase) GetScoreHistory(ctx context.Context, entityRef models.ScoringEntityRef) ([]models.ScoringScore, error) {
	entityRef.OrgId = uc.enforceSecurity.OrgId()

	scores, err := uc.repository.GetScoreHistory(ctx, uc.executorFactory.NewExecutor(), entityRef)
	if err != nil {
		return nil, err
	}

	for _, score := range scores {
		if err := uc.enforceSecurity.ReadEntityScore(score); err != nil {
			return nil, err
		}
	}

	return scores, nil
}

func (uc ScoringScoresUsecase) GetActiveScore(ctx context.Context, entity models.ScoringEntityRef, opts models.RefreshScoreOptions) (*models.ScoringScore, error) {
	entity.OrgId = uc.enforceSecurity.OrgId()

	score, err := uc.repository.GetActiveScore(ctx, uc.executorFactory.NewExecutor(), entity)
	if err != nil {
		return nil, err
	}

	score, err = uc.tryRefreshScore(ctx, score, entity, opts)
	if err != nil {
		return nil, err
	}
	if score == nil {
		return &models.ScoringScore{}, errors.Wrap(models.NotFoundError, "no score was found for this entity")
	}

	if err := uc.enforceSecurity.ReadEntityScore(*score); err != nil {
		return nil, err
	}

	return score, nil
}

func (uc ScoringScoresUsecase) tryRefreshScore(ctx context.Context, activeScore *models.ScoringScore, entity models.ScoringEntityRef, opts models.RefreshScoreOptions) (*models.ScoringScore, error) {
	return executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) (*models.ScoringScore, error) {
		// We do not compute the score in the background if we do not have a
		// currently active score. In this case, we fall back to synchronous
		// computing.
		//
		// If we trigger the refresh in the background, we return the current score.
		if activeScore != nil && opts.RefreshInBackground {
			if activeScore.CreatedAt.Add(opts.RefreshOlderThan).Before(time.Now()) {
				if err := uc.taskQueueRepository.EnqueueTriggerScoreComputation(ctx, tx, entity); err != nil {
					return nil, err
				}
			}

			return activeScore, nil
		}

		if !activeScore.IsStale(opts.RefreshOlderThan) {
			return activeScore, nil
		}

		// In case an error is encountered while synchronously refreshing the
		// score, log it but return the current score, if there is one.
		newScore, err := uc.ComputeScore(ctx, entity.EntityType, entity.EntityId)
		if err != nil {
			if activeScore == nil {
				return nil, err
			}

			utils.LoggerFromContext(ctx).ErrorContext(ctx,
				"could not synchronously refresh user score",
				"error", err.Error())

			return activeScore, nil
		}
		if newScore == nil {
			return activeScore, nil
		}

		req := models.InsertScoreRequest{
			OrgId:      entity.OrgId,
			EntityType: entity.EntityType,
			EntityId:   entity.EntityId,
			Score:      newScore.Score,
			Source:     models.ScoreSourceRuleset,
		}

		score, err := uc.repository.InsertScore(ctx, tx, req)
		if err != nil {
			if activeScore == nil {
				return nil, err
			}

			utils.LoggerFromContext(ctx).ErrorContext(ctx,
				"could not synchronously refresh user score",
				"error", err.Error())

			return activeScore, nil
		}

		return &score, nil
	})
}

func (uc ScoringScoresUsecase) OverrideScore(ctx context.Context, req models.InsertScoreRequest) (models.ScoringScore, error) {
	exec := uc.executorFactory.NewExecutor()

	req.OrgId = uc.enforceSecurity.OrgId()

	if req.Source == models.ScoreSourceOverride {
		switch {
		case uc.enforceSecurity.UserId() != nil:
			req.OverridenBy = utils.Ptr(uuid.MustParse(*uc.enforceSecurity.UserId()))
		case uc.enforceSecurity.ApiKeyId() != nil:
			req.OverridenBy = utils.Ptr(uuid.MustParse(*uc.enforceSecurity.ApiKeyId()))
		}

		dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, req.OrgId, false, false)
		if err != nil {
			return models.ScoringScore{}, err
		}

		if _, err := uc.getPayloadObject(ctx, req.OrgId, dataModel, req.EntityType, req.EntityId); err != nil {
			return models.ScoringScore{}, err
		}

		if err := uc.enforceSecurity.OverrideScore(req.ToEntityRef()); err != nil {
			return models.ScoringScore{}, err
		}
	}

	score, err := executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) (models.ScoringScore, error) {
		return uc.repository.InsertScore(ctx, tx, req)
	})

	return score, err
}

func (uc ScoringScoresUsecase) getPayloadObject(ctx context.Context, orgId uuid.UUID, dataModel models.DataModel, entityType, entityId string) (models.ClientObject, error) {
	table, ok := dataModel.Tables[entityType]
	if !ok {
		return models.ClientObject{}, errors.Newf("unknown entity type '%s'", entityType)
	}

	clientExec, err := uc.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return models.ClientObject{}, err
	}

	objs, err := uc.ingestedDataReader.QueryIngestedObject(ctx, clientExec, table, entityId)
	if err != nil {
		return models.ClientObject{}, err
	}
	if len(objs) == 0 {
		return models.ClientObject{}, errors.Wrap(models.NotFoundError, "unknown entity")
	}

	return models.ClientObject{
		TableName: table.Name,
		Data:      objs[0].Data,
	}, nil
}

func (uc ScoringScoresUsecase) executeRules(ctx context.Context, env ast_eval.AstEvaluationEnvironment, ruleset models.ScoringRuleset) (models.ScoringEvaluation, error) {
	score := models.ScoringEvaluation{
		Evaluation: make([]ast.NodeEvaluation, len(ruleset.Rules)),
	}

	for idx, rule := range ruleset.Rules {
		eval, ok := ast_eval.EvaluateAst(ctx, nil, env, rule.Ast)
		if !ok {
			return models.ScoringEvaluation{}, errors.New("could not unmarshal AST")
		}

		scoreComputationResult, ok := eval.ReturnValue.(ast.ScoreComputationResult)
		if !ok {
			return models.ScoringEvaluation{}, errors.New("AST was not a score computation result")
		}

		score.Evaluation[idx] = eval

		if !scoreComputationResult.Triggered {
			continue
		}

		score.Modifier += scoreComputationResult.Modifier
		score.Floor = max(score.Floor, scoreComputationResult.Floor)
	}

	return score, nil
}

func (uc ScoringScoresUsecase) internalScoreToScore(ruleset models.ScoringRuleset, eval models.ScoringEvaluation) int {
	thresholds := make([]int, 0, len(ruleset.Thresholds)+1)
	thresholds = append(thresholds, ruleset.Thresholds...)
	thresholds = append(thresholds, 1<<32)

	score := 0

	for idx, threshold := range thresholds {
		if threshold > eval.Modifier {
			if idx == len(ruleset.Thresholds) {
				score += 1
			}

			break
		}

		score = idx
	}

	return max(score+1, eval.Floor)
}
