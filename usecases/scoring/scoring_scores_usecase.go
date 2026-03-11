package scoring

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
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
	offloadedReadWriter repositories.OffloadedReadWriter
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
	offloadedReadWriter repositories.OffloadedReadWriter,
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
		offloadedReadWriter: offloadedReadWriter,
		ingestedDataReader:  ingestedDataReader,
		taskQueueRepository: taskQueueRepository,
		evaluateAst:         evaluateAst,
	}
}

func (uc ScoringScoresUsecase) ComputeScore(ctx context.Context, orgId uuid.UUID, recordType, recordId string) (models.ScoringRuleset, *models.ScoringEvaluation, error) {
	exec := uc.executorFactory.NewExecutor()

	ruleset, err := uc.repository.GetScoringRuleset(
		ctx,
		exec,
		orgId,
		recordType,
		models.ScoreRulesetCommitted,
		0)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			return models.ScoringRuleset{}, nil, errors.Wrap(err, "no committed version of this ruleset")
		}

		return models.ScoringRuleset{}, nil, err
	}

	if err := uc.enforceSecurity.ReadOrganization(ruleset.OrgId); err != nil {
		return models.ScoringRuleset{}, nil, err
	}

	eval, err := uc.InternalComputeScore(ctx, exec, orgId, ruleset, recordType, recordId)
	if err != nil {
		return ruleset, nil, err
	}

	return ruleset, eval, nil
}

func (uc ScoringScoresUsecase) InternalComputeScore(ctx context.Context, exec repositories.Executor, orgId uuid.UUID,
	ruleset models.ScoringRuleset,
	recordType, recordId string,
) (*models.ScoringEvaluation, error) {
	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, orgId, false, false)
	if err != nil {
		return nil, err
	}

	object, err := uc.getPayloadObject(ctx, orgId, dataModel, recordType, recordId)
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

	eval.RiskLevel = uc.scoreToRiskLevel(ruleset, eval)

	return &eval, nil
}

func (uc ScoringScoresUsecase) GetScoreHistory(ctx context.Context, record models.ScoringRecordRef) ([]models.ScoringScore, error) {
	record.OrgId = uc.enforceSecurity.OrgId()

	scores, err := uc.repository.GetScoreHistory(ctx, uc.executorFactory.NewExecutor(), record)
	if err != nil {
		return nil, err
	}

	for _, score := range scores {
		if err := uc.enforceSecurity.ReadRecordScore(score); err != nil {
			return nil, err
		}
	}

	return scores, nil
}

func (uc ScoringScoresUsecase) GetActiveScore(ctx context.Context, record models.ScoringRecordRef, withEvaluation bool, opts models.RefreshScoreOptions) (*models.ScoringScore, []*ast.NodeEvaluationDto, error) {
	exec := uc.executorFactory.NewExecutor()

	score, err := uc.repository.GetActiveScore(ctx, exec, record)
	if err != nil {
		return nil, nil, err
	}

	score, err = uc.tryRefreshScore(ctx, score, record, opts)
	if err != nil {
		return nil, nil, err
	}
	if score == nil {
		return nil, nil, errors.Wrap(models.NotFoundError, "no score was found for this record")
	}

	if err := uc.enforceSecurity.ReadRecordScore(*score); err != nil {
		return nil, nil, err
	}

	if score.RulesetId == nil {
		return score, nil, nil
	}

	var ruleEvaluations []*ast.NodeEvaluationDto

	if withEvaluation {
		ruleset, err := uc.repository.GetScoringRulesetById(ctx, exec, record.OrgId, *score.RulesetId)
		if err != nil {
			return nil, nil, err
		}

		ruleEvaluations, err = uc.offloadedReadWriter.GetOffloadedScoreComputation(ctx, exec, record.OrgId, ruleset, *score)
		if err != nil {
			return nil, nil, err
		}
	}

	return score, ruleEvaluations, nil
}

func (uc ScoringScoresUsecase) tryRefreshScore(ctx context.Context, activeScore *models.ScoringScore, record models.ScoringRecordRef, opts models.RefreshScoreOptions) (*models.ScoringScore, error) {
	return executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) (*models.ScoringScore, error) {
		// We do not compute the score in the background if we do not have a
		// currently active score. In this case, we fall back to synchronous
		// computing.
		//
		// If we trigger the refresh in the background, we return the current score.
		if activeScore != nil && opts.RefreshInBackground {
			if activeScore.CreatedAt.Add(opts.RefreshOlderThan).Before(time.Now()) {
				if err := uc.taskQueueRepository.EnqueueTriggerScoreComputation(ctx, tx, record); err != nil {
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
		scoreRuleset, newScore, err := uc.ComputeScore(ctx, record.OrgId, record.RecordType, record.RecordId)
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
			OrgId:      record.OrgId,
			RecordType: record.RecordType,
			RecordId:   record.RecordId,
			RiskLevel:  newScore.RiskLevel,
			Source:     models.ScoreSourceRuleset,
			RulesetId:  &scoreRuleset.Id,
		}

		if activeScore != nil && newScore.RiskLevel < activeScore.RiskLevel {
			if activeScore.CreatedAt.Add(time.Duration(scoreRuleset.CooldownSeconds) * time.Second).After(time.Now()) {
				req.IgnoredByCooldown = true
			}
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

		scoreEvaluations := pure_utils.Map(newScore.Evaluation, func(ne ast.NodeEvaluation) *ast.NodeEvaluationDto {
			return utils.Ptr(ast.AdaptNodeEvaluationDto(ne))
		})

		scoreEvaluationsSer, err := dbmodels.SerializeDecisionEvaluationDto(scoreEvaluations)
		if err != nil {
			return nil, err
		}

		if err := uc.offloadedReadWriter.OffloadScoreComputation(ctx, scoreRuleset, score, scoreEvaluationsSer); err != nil {
			return nil, errors.Wrap(err, "could not offload score computation")
		}

		return &score, nil
	})
}

func (uc ScoringScoresUsecase) OverrideScore(ctx context.Context, req models.InsertScoreRequest) (models.ScoringScore, error) {
	exec := uc.executorFactory.NewExecutor()

	req.OrgId = uc.enforceSecurity.OrgId()

	if err := uc.enforceSecurity.OverrideScore(req.ToRecordRef()); err != nil {
		return models.ScoringScore{}, err
	}

	settings, err := uc.repository.GetScoringSettings(ctx, exec, req.OrgId)
	if err != nil {
		return models.ScoringScore{}, err
	}
	if settings == nil {
		return models.ScoringScore{}, errors.Wrap(models.BadParameterError, "no global scoring settings for this organization")
	}

	if req.RiskLevel < 1 || req.RiskLevel > settings.MaxRiskLevel {
		return models.ScoringScore{}, errors.Wrapf(models.BadParameterError, "expected risk level in range 1-%d", settings.MaxRiskLevel)
	}

	if req.Source == models.ScoreSourceOverride {
		switch {
		case uc.enforceSecurity.UserId() != nil:
			req.OverriddenBy = utils.Ptr(uuid.MustParse(*uc.enforceSecurity.UserId()))
		case uc.enforceSecurity.ApiKeyId() != nil:
			req.OverriddenBy = utils.Ptr(uuid.MustParse(*uc.enforceSecurity.ApiKeyId()))
		}

		dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, req.OrgId, false, false)
		if err != nil {
			return models.ScoringScore{}, err
		}

		if _, err := uc.getPayloadObject(ctx, req.OrgId, dataModel, req.RecordType, req.RecordId); err != nil {
			return models.ScoringScore{}, err
		}
	}

	score, err := executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) (models.ScoringScore, error) {
		return uc.repository.InsertScore(ctx, tx, req)
	})

	return score, err
}

func (uc ScoringScoresUsecase) GetScoreDistribution(ctx context.Context, entityType string) ([]models.ScoreDistribution, error) {
	exec := uc.executorFactory.NewExecutor()
	orgId := uc.enforceSecurity.OrgId()

	if err := uc.enforceSecurity.ReadOrganization(orgId); err != nil {
		return nil, err
	}

	return uc.repository.GetScoreDistribution(ctx, exec, orgId, entityType)
}

func (uc ScoringScoresUsecase) getPayloadObject(ctx context.Context, orgId uuid.UUID, dataModel models.DataModel, recordType, recordId string) (models.ClientObject, error) {
	table, ok := dataModel.Tables[recordType]
	if !ok {
		return models.ClientObject{}, errors.Newf("unknown record type '%s'", recordType)
	}

	clientExec, err := uc.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return models.ClientObject{}, err
	}

	objs, err := uc.ingestedDataReader.QueryIngestedObject(ctx, clientExec, table, recordId)
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

func (uc ScoringScoresUsecase) scoreToRiskLevel(ruleset models.ScoringRuleset, eval models.ScoringEvaluation) int {
	riskLevel := 1

	for _, threshold := range ruleset.Thresholds {
		if eval.Modifier < threshold {
			break
		}
		riskLevel++
	}

	return max(riskLevel, eval.Floor)
}
