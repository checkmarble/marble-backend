package scoring

import (
	"context"
	"encoding/json"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/dto/scoring"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
)

type ScoringRulesetsUsecase struct {
	enforceSecurity     security.EnforceSecurityScoring
	executorFactory     executor_factory.ExecutorFactory
	transactionFactory  executor_factory.TransactionFactory
	repository          ScoringRepository
	indexEditor         scoringIndexEditor
	taskQueueRepository repositories.TaskQueueRepository
}

func NewScoringRulesetsUsecase(
	enforceSecurity security.EnforceSecurityScoring,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repository ScoringRepository,
	indexEditor scoringIndexEditor,
	taskQueueRepository repositories.TaskQueueRepository,
) ScoringRulesetsUsecase {
	return ScoringRulesetsUsecase{
		enforceSecurity:     enforceSecurity,
		executorFactory:     executorFactory,
		transactionFactory:  transactionFactory,
		repository:          repository,
		indexEditor:         indexEditor,
		taskQueueRepository: taskQueueRepository,
	}
}

func (uc ScoringRulesetsUsecase) ListRulesets(ctx context.Context) ([]models.ScoringRuleset, error) {
	rulesets, err := uc.repository.ListScoringRulesets(ctx, uc.executorFactory.NewExecutor(), uc.enforceSecurity.OrgId())
	if err != nil {
		return nil, err
	}

	return rulesets, err
}

func (uc ScoringRulesetsUsecase) GetRuleset(ctx context.Context, entityType string, status models.ScoreRulesetStatus) (models.ScoringRuleset, error) {
	ruleset, err := uc.repository.GetScoringRuleset(
		ctx,
		uc.executorFactory.NewExecutor(),
		uc.enforceSecurity.OrgId(),
		entityType,
		status)
	if err != nil {
		return models.ScoringRuleset{}, err
	}

	return ruleset, err
}

func (uc ScoringRulesetsUsecase) CreateRulesetVersion(ctx context.Context, entityType string, req scoring.CreateRulesetRequest) (models.ScoringRuleset, error) {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.UpdateRuleset(orgId); err != nil {
		return models.ScoringRuleset{}, err
	}

	for _, rule := range req.Rules {
		if err := uc.validateScoringRuleAst(rule.Ast); err != nil {
			return models.ScoringRuleset{}, errors.Wrap(models.BadParameterError, err.Error())
		}
	}

	existingRuleset, err := uc.repository.GetScoringRuleset(ctx, exec, orgId, entityType, models.ScoreRulesetCommitted)
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

		if err := uc.repository.DeleteAllRulesetRules(ctx, tx, ruleset); err != nil {
			return models.ScoringRuleset{}, err
		}

		for idx, rreq := range req.Rules {
			ser, err := json.Marshal(rreq.Ast)
			if err != nil {
				return models.ScoringRuleset{}, err
			}

			r := models.CreateScoringRuleRequest{
				StableId:    rreq.StableId,
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

func (uc ScoringRulesetsUsecase) PreparationStatus(ctx context.Context, entityType string) (models.PublicationPreparationStatus, error) {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.UpdateRuleset(orgId); err != nil {
		return models.PublicationPreparationStatus{}, err
	}

	draft, err := uc.repository.GetScoringRuleset(ctx, exec, orgId, entityType, models.ScoreRulesetDraft)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			return models.PublicationPreparationStatus{}, errors.Wrap(err, "no draft version found")
		}

		return models.PublicationPreparationStatus{}, err
	}

	indexes, pending, err := uc.indexEditor.GetIndexesToCreateForScoringRuleset(ctx, orgId, draft)
	if err != nil {
		return models.PublicationPreparationStatus{}, err
	}

	status := models.PublicationPreparationStatus{
		PreparationStatus:        models.PreparationStatusReadyToActivate,
		PreparationServiceStatus: models.PreparationServiceStatusAvailable,
	}

	if len(indexes) > 0 {
		status.PreparationStatus = models.PreparationStatusRequired
	}
	if pending > 0 {
		status.PreparationServiceStatus = models.PreparationServiceStatusOccupied
	}

	return status, nil
}

func (uc ScoringRulesetsUsecase) PrepareRuleset(ctx context.Context, entityType string) error {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.UpdateRuleset(orgId); err != nil {
		return err
	}

	draft, err := uc.repository.GetScoringRuleset(ctx, exec, orgId, entityType, models.ScoreRulesetDraft)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			return errors.Wrap(err, "no draft version found")
		}

		return err
	}

	indexes, pending, err := uc.indexEditor.GetIndexesToCreateForScoringRuleset(ctx, orgId, draft)
	if err != nil {
		return err
	}

	if pending > 0 {
		return errors.Wrap(models.UnprocessableEntityError, "ruleset is still being prepared")
	}

	if len(indexes) > 0 {
		if err := uc.taskQueueRepository.EnqueueCreateIndexTask(ctx, orgId, indexes); err != nil {
			return err
		}
	}

	return nil
}

func (uc ScoringRulesetsUsecase) CommitRuleset(ctx context.Context, entityType string) (models.ScoringRuleset, error) {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.UpdateRuleset(orgId); err != nil {
		return models.ScoringRuleset{}, err
	}

	ruleset, err := executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) (models.ScoringRuleset, error) {
		draft, err := uc.repository.GetScoringRuleset(ctx, exec, orgId, entityType, models.ScoreRulesetDraft)
		if err != nil {
			if errors.Is(err, models.NotFoundError) {
				return models.ScoringRuleset{}, errors.Wrap(err, "no draft version found")
			}

			return models.ScoringRuleset{}, err
		}

		indexes, pending, err := uc.indexEditor.GetIndexesToCreateForScoringRuleset(ctx, orgId, draft)
		if err != nil {
			return models.ScoringRuleset{}, err
		}

		if pending > 0 {
			return models.ScoringRuleset{}, errors.Wrap(models.UnprocessableEntityError, "ruleset is still being prepared")
		}
		if len(indexes) > 0 {
			return models.ScoringRuleset{}, errors.Wrap(models.UnprocessableEntityError, "ruleset is not prepared")
		}

		return uc.repository.CommitRuleset(ctx, exec, draft)
	})

	return ruleset, err
}

func (uc ScoringRulesetsUsecase) validateScoringRuleAst(tree dto.NodeDto) error {
	if tree.Name == ast.FuncAttributesMap[ast.FUNC_SCORE_COMPUTATION].AstName {
		return nil
	}

	if tree.Name == ast.FuncAttributesMap[ast.FUNC_SWITCH].AstName {
		if len(tree.Children) == 0 {
			return errors.New("invalid root AST node for user scoring: `Switch` must contain at least one child")
		}

		for _, child := range tree.Children {
			if child.Name != ast.FuncAttributesMap[ast.FUNC_SCORE_COMPUTATION].AstName {
				return errors.New("invalid root AST node for user scoring: all `Switch` children must be `ScoreComputation`")
			}
		}

		return nil
	}

	return errors.New("invalid AST root node for user scoring: must be `ScoreComputation` or `Switch`")
}
