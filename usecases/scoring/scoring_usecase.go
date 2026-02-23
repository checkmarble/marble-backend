package scoring

import (
	"context"
	"encoding/json"

	"github.com/checkmarble/marble-backend/dto/scoring"
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

type scoringRepository interface {
	GetScoringRuleset(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, entityType string) (models.ScoringRuleset, error)
	InsertScoringRulesetVersion(ctx context.Context, exec repositories.Transaction,
		orgId uuid.UUID,
		ruleset models.CreateScoringRulesetRequest,
	) (models.ScoringRuleset, error)
	InsertScoringRulesetVersionRule(ctx context.Context, tx repositories.Transaction,
		ruleset models.ScoringRuleset,
		rule models.CreateScoringRuleRequest,
	) (models.ScoringRule, error)

	GetScoreHistory(ctx context.Context, exec repositories.Executor, entityRef models.ScoringEntityRef) ([]models.ScoringScore, error)
	GetActiveScore(ctx context.Context, exec repositories.Executor, entityRef models.ScoringEntityRef) (*models.ScoringScore, error)
	InsertScore(ctx context.Context, tx repositories.Transaction, req models.InsertScoreRequest) (models.ScoringScore, error)
}

type scoringIngestedDataReader interface {
	QueryIngestedObject(ctx context.Context, exec repositories.Executor,
		table models.Table, objectId string, metadataFields ...string) ([]models.DataModelObject, error)
}

type ScoringUsecase struct {
	enforceSecurity     security.EnforceSecurityScoring
	executorFactory     executor_factory.ExecutorFactory
	transactionFactory  executor_factory.TransactionFactory
	repository          scoringRepository
	dataModelRepository repositories.DataModelRepository
	ingestedDataReader  scoringIngestedDataReader
	evaluateAst         ast_eval.EvaluateAstExpression
}

func NewScoringUsecase(
	enforceSecurity security.EnforceSecurityScoring,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repository scoringRepository,
	dataModelRepository repositories.DataModelRepository,
	ingestedDataReader scoringIngestedDataReader,
	evaluateAst ast_eval.EvaluateAstExpression,
) ScoringUsecase {
	return ScoringUsecase{
		enforceSecurity:     enforceSecurity,
		executorFactory:     executorFactory,
		transactionFactory:  transactionFactory,
		repository:          repository,
		dataModelRepository: dataModelRepository,
		ingestedDataReader:  ingestedDataReader,
		evaluateAst:         evaluateAst,
	}
}

func (uc ScoringUsecase) TestScore(ctx context.Context, entityType, entityId string) (models.ScoringEvaluation, error) {
	ruleset, err := uc.repository.GetScoringRuleset(
		ctx,
		uc.executorFactory.NewExecutor(),
		uc.enforceSecurity.OrgId(),
		entityType)
	if err != nil {
		return models.ScoringEvaluation{}, err
	}

	exec := uc.executorFactory.NewExecutor()
	orgId := uc.enforceSecurity.OrgId()

	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, orgId, false, false)
	if err != nil {
		return models.ScoringEvaluation{}, err
	}

	object, err := uc.getPayloadObject(ctx, orgId, dataModel, entityType, entityId)
	if err != nil {
		return models.ScoringEvaluation{}, err
	}

	env := uc.evaluateAst.AstEvaluationEnvironmentFactory(ast_eval.EvaluationEnvironmentFactoryParams{
		OrganizationId: orgId,
		DataModel:      dataModel,
		ClientObject:   object,
	})

	eval, err := uc.executeRules(ctx, env, ruleset)
	if err != nil {
		return models.ScoringEvaluation{}, err
	}

	eval.Score = uc.internalScoreToScore(ruleset, eval)

	return eval, nil
}

func (uc ScoringUsecase) GetRuleset(ctx context.Context, entityType string) (models.ScoringRuleset, error) {
	ruleset, err := uc.repository.GetScoringRuleset(
		ctx,
		uc.executorFactory.NewExecutor(),
		uc.enforceSecurity.OrgId(),
		entityType)
	if err != nil {
		return models.ScoringRuleset{}, err
	}

	return ruleset, err
}

func (uc ScoringUsecase) CreateRulesetVersion(ctx context.Context, entityType string, req scoring.CreateRulesetRequest) (models.ScoringRuleset, error) {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	existingRuleset, err := uc.repository.GetScoringRuleset(ctx, exec, orgId, entityType)
	if err != nil && !errors.Is(err, models.NotFoundError) {
		return models.ScoringRuleset{}, err
	}

	ruleset, err := executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) (models.ScoringRuleset, error) {
		rs := models.CreateScoringRulesetRequest{
			Version:         existingRuleset.Version + 1,
			Name:            req.Name,
			Description:     req.Description,
			EntityType:      entityType,
			Thresholds:      req.Thresholds,
			CooldownSeconds: req.CooldownSeconds,
		}

		ruleset, err := uc.repository.InsertScoringRulesetVersion(ctx, tx, orgId, rs)
		if err != nil {
			return models.ScoringRuleset{}, err
		}

		ruleset.Rules = make([]models.ScoringRule, len(req.Rules))

		for idx, rreq := range req.Rules {
			ser, err := json.Marshal(rreq.Ast)
			if err != nil {
				return models.ScoringRuleset{}, err
			}

			r := models.CreateScoringRuleRequest{
				Name:        rreq.Name,
				Description: rreq.Description,
				Ast:         ser,
			}

			rule, err := uc.repository.InsertScoringRulesetVersionRule(ctx, tx, ruleset, r)
			if err != nil {
				return models.ScoringRuleset{}, err
			}

			ruleset.Rules[idx] = rule
		}

		return ruleset, nil
	})

	return ruleset, err
}

func (uc ScoringUsecase) GetScoreHistory(ctx context.Context, entityRef models.ScoringEntityRef) ([]models.ScoringScore, error) {
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

func (uc ScoringUsecase) GetActiveScore(ctx context.Context, entityRef models.ScoringEntityRef) (*models.ScoringScore, error) {
	entityRef.OrgId = uc.enforceSecurity.OrgId()

	score, err := uc.repository.GetActiveScore(ctx, uc.executorFactory.NewExecutor(), entityRef)
	if err != nil || score == nil {
		return nil, err
	}

	if err := uc.enforceSecurity.ReadEntityScore(*score); err != nil {
		return nil, err
	}

	return score, nil
}

func (uc ScoringUsecase) OverrideScore(ctx context.Context, req models.InsertScoreRequest) (models.ScoringScore, error) {
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

func (uc ScoringUsecase) ComputeScore(ctx context.Context, req models.InsertScoreRequest) (models.ScoringScore, error) {
	return models.ScoringScore{}, errors.New("not yet implemented")
}

func (uc ScoringUsecase) getPayloadObject(ctx context.Context, orgId uuid.UUID, dataModel models.DataModel, entityType, entityId string) (models.ClientObject, error) {
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

func (uc ScoringUsecase) executeRules(ctx context.Context, env ast_eval.AstEvaluationEnvironment, ruleset models.ScoringRuleset) (models.ScoringEvaluation, error) {
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

func (uc ScoringUsecase) internalScoreToScore(ruleset models.ScoringRuleset, eval models.ScoringEvaluation) int {
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
