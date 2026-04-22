package scoring

import (
	"context"
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/dto/scoring"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/feature_access"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const RULESET_DRY_RUN_SAMPLE_SIZE = 5000

type ScoringRulesetsUsecase struct {
	enforceSecurity     security.EnforceSecurityScoring
	executorFactory     executor_factory.ExecutorFactory
	transactionFactory  executor_factory.TransactionFactory
	featureAccessReader featureAccessReader
	redisClient         *repositories.RedisClient
	repository          ScoringRepository
	indexEditor         scoringIndexEditor
	taskQueueRepository repositories.TaskQueueRepository
	validateScenarioAst scenarios.ValidateScenarioAst
}

func NewScoringRulesetsUsecase(
	enforceSecurity security.EnforceSecurityScoring,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	featureAccessReader feature_access.FeatureAccessReader,
	redisClient *repositories.RedisClient,
	repository ScoringRepository,
	indexEditor scoringIndexEditor,
	taskQueueRepository repositories.TaskQueueRepository,
	validateScenarioAst scenarios.ValidateScenarioAst,
) ScoringRulesetsUsecase {
	return ScoringRulesetsUsecase{
		enforceSecurity:     enforceSecurity,
		executorFactory:     executorFactory,
		transactionFactory:  transactionFactory,
		featureAccessReader: featureAccessReader,
		redisClient:         redisClient,
		repository:          repository,
		indexEditor:         indexEditor,
		taskQueueRepository: taskQueueRepository,
		validateScenarioAst: validateScenarioAst,
	}
}

func (uc ScoringRulesetsUsecase) CommittedRulesetExists(ctx context.Context, orgId uuid.UUID, recordType string) (bool, error) {
	cache := uc.redisClient.NewExecutor(orgId)
	cacheKey := cache.Key("scoring_record_types", recordType)

	exists, err := repositories.RedisQuery(cache, func(c *redis.Client) (bool, error) {
		result := c.Get(ctx, cacheKey)

		if err := result.Err(); err != nil {
			return false, err
		}

		return result.Bool()
	})

	if err == nil {
		return exists, nil
	}

	exists = true

	_, err = uc.repository.GetScoringRuleset(ctx, uc.executorFactory.NewExecutor(), orgId, recordType, models.ScoreRulesetCommitted, 0)
	if err != nil {
		if !errors.Is(err, models.NotFoundError) {
			return false, err
		}

		exists = false
	}

	_ = cache.Exec(func(c *redis.Client) error {
		return c.Set(ctx, cacheKey, exists, time.Hour).Err()
	})

	return exists, nil
}

func (uc ScoringRulesetsUsecase) ListRulesets(ctx context.Context) ([]models.ScoringRuleset, error) {
	rulesets, err := uc.repository.ListScoringRulesets(ctx, uc.executorFactory.NewExecutor(), uc.enforceSecurity.OrgId())
	if err != nil {
		return nil, err
	}

	for _, ruleset := range rulesets {
		if err := uc.enforceSecurity.ReadOrganization(ruleset.OrgId); err != nil {
			return nil, err
		}
	}

	return rulesets, err
}

func (uc ScoringRulesetsUsecase) GetRuleset(ctx context.Context, recordType string, status models.ScoreRulesetStatus, version int) (models.ScoringRuleset, error) {
	exec := uc.redisClient.NewExecutor(uc.enforceSecurity.OrgId())
	cacheKey := exec.Key("scoring_record_types", recordType)

	ruleset, err := uc.repository.GetScoringRuleset(
		ctx,
		uc.executorFactory.NewExecutor(),
		uc.enforceSecurity.OrgId(),
		recordType,
		status, version)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			_ = exec.Exec(func(c *redis.Client) error {
				return c.Set(ctx, cacheKey, false, time.Hour).Err()
			})
		}

		return models.ScoringRuleset{}, err
	}

	if err := uc.enforceSecurity.ReadOrganization(ruleset.OrgId); err != nil {
		return models.ScoringRuleset{}, err
	}

	if ruleset.Status == models.ScoreRulesetCommitted {
		cacheKey := exec.Key("scoring_record_types", ruleset.RecordType)

		_ = exec.Exec(func(c *redis.Client) error {
			return c.Set(ctx, cacheKey, true, time.Hour).Err()
		})
	}

	return ruleset, err
}

func (uc ScoringRulesetsUsecase) ListRulesetVersions(ctx context.Context, recordType string) ([]models.ScoringRuleset, error) {
	rulesets, err := uc.repository.ListScoringRulesetVersions(
		ctx,
		uc.executorFactory.NewExecutor(),
		uc.enforceSecurity.OrgId(),
		recordType)
	if err != nil {
		return nil, err
	}

	return rulesets, err
}

func (uc ScoringRulesetsUsecase) CreateRulesetVersion(ctx context.Context, recordType string, req scoring.CreateRulesetRequest) (models.ScoringRuleset, error) {
	orgId := uc.enforceSecurity.OrgId()

	if err := uc.isScoringEnabled(ctx, orgId); err != nil {
		return models.ScoringRuleset{}, err
	}

	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.UpdateRuleset(orgId); err != nil {
		return models.ScoringRuleset{}, err
	}

	settings, err := uc.repository.GetScoringSettings(ctx, exec, orgId)
	if err != nil {
		return models.ScoringRuleset{}, err
	}
	if settings == nil {
		return models.ScoringRuleset{}, errors.Wrap(models.BadParameterError, "no global scoring settings for this organization")
	}

	// We need a number of thresholds == (max_level - 1) for the score mapping to make sense:
	//   T-1 | T-2
	// 1  <  2  <  3
	if len(req.Thresholds) != settings.MaxRiskLevel-1 {
		return models.ScoringRuleset{}, errors.Wrapf(models.BadParameterError,
			"invalid thresholds, expected a list of %d (max score is %d)", settings.MaxRiskLevel-1, settings.MaxRiskLevel)
	}

	for _, rule := range req.Rules {
		if err := uc.validateScoringRuleAst(rule.Ast); err != nil {
			return models.ScoringRuleset{}, errors.Wrap(models.BadParameterError, err.Error())
		}

		astNode, err := dto.AdaptASTNode(rule.Ast)
		if err != nil {
			return models.ScoringRuleset{}, err
		}

		astValidation, err := uc.ValidateAst(ctx, recordType, &astNode)
		if err != nil {
			return models.ScoringRuleset{}, err
		}
		if len(astValidation.Errors) > 0 {
			errs := pure_utils.Map(astValidation.Errors, func(e models.ScenarioValidationError) error {
				return e.Error
			})

			return models.ScoringRuleset{}, errors.Wrap(models.BadParameterError, errors.Join(errs...).Error())
		}
	}

	existingRuleset, err := uc.repository.GetScoringRuleset(ctx, exec, orgId, recordType, models.ScoreRulesetCommitted, 0)
	if err != nil && !errors.Is(err, models.NotFoundError) {
		return models.ScoringRuleset{}, err
	}

	ruleset, err := executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) (models.ScoringRuleset, error) {
		rs := models.CreateScoringRulesetRequest{
			Version:                existingRuleset.Version + 1,
			Name:                   req.Name,
			Description:            req.Description,
			RecordType:             recordType,
			Thresholds:             req.Thresholds,
			CooldownSeconds:        req.CooldownSeconds,
			ScoringIntervalSeconds: req.ScoringIntervalSeconds,
		}

		ruleset, err := uc.repository.InsertScoringRulesetVersion(ctx, tx, orgId, rs)
		if err != nil {
			return models.ScoringRuleset{}, err
		}

		rulesReq := make([]models.CreateScoringRuleRequest, len(req.Rules))

		for idx, r := range req.Rules {
			ser, err := json.Marshal(r.Ast)
			if err != nil {
				return models.ScoringRuleset{}, err
			}

			riskType := models.ScoringRuleRiskTypeFrom(r.RiskType)
			if riskType == models.ScoringRiskUnknown {
				return models.ScoringRuleset{}, errors.Wrapf(models.BadParameterError, "invalid risk type '%s'", r.RiskType)
			}

			rulesReq[idx] = models.CreateScoringRuleRequest{
				StableId:    r.StableId,
				Name:        r.Name,
				Description: r.Description,
				RiskType:    riskType,
				Ast:         ser,
			}
		}

		if existingRuleset.Id != uuid.Nil {
			if err := uc.repository.CancelRulesetDryRun(ctx, tx, existingRuleset); err != nil {
				return models.ScoringRuleset{}, err
			}
		}

		rules, err := uc.repository.InsertScoringRulesetVersionRule(ctx, tx, ruleset, rulesReq)
		if err != nil {
			if repositories.IsUniqueViolationError(err) {
				return models.ScoringRuleset{}, errors.Wrap(models.BadParameterError, "duplicate rule IDs")
			}

			return models.ScoringRuleset{}, err
		}

		ruleset.Rules = rules

		return ruleset, nil
	})

	return ruleset, err
}

func (uc ScoringRulesetsUsecase) PreparationStatus(ctx context.Context, recordType string) (models.PublicationPreparationStatus, error) {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.UpdateRuleset(orgId); err != nil {
		return models.PublicationPreparationStatus{}, err
	}

	draft, err := uc.repository.GetScoringRuleset(ctx, exec, orgId, recordType, models.ScoreRulesetDraft, 0)
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

func (uc ScoringRulesetsUsecase) PrepareRuleset(ctx context.Context, recordType string) error {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.UpdateRuleset(orgId); err != nil {
		return err
	}

	draft, err := uc.repository.GetScoringRuleset(ctx, exec, orgId, recordType, models.ScoreRulesetDraft, 0)
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
		if err := uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
			return uc.taskQueueRepository.EnqueueCreateIndexTask(ctx, tx, orgId, indexes)
		}); err != nil {
			return err
		}
	}

	return nil
}

func (uc ScoringRulesetsUsecase) GetDryRun(ctx context.Context, recordType string) (models.ScoringDryRun, error) {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	draft, err := uc.repository.GetScoringRuleset(ctx, exec, orgId, recordType, models.ScoreRulesetDraft, 0)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			return models.ScoringDryRun{}, errors.Wrap(err, "no draft version found")
		}

		return models.ScoringDryRun{}, err
	}

	if err := uc.enforceSecurity.UpdateRuleset(orgId); err != nil {
		return models.ScoringDryRun{}, err
	}

	return uc.repository.GetScoringLatestDryRun(ctx, exec, draft.Id)
}

func (uc ScoringRulesetsUsecase) StartDryRun(ctx context.Context, recordType string) (models.ScoringDryRun, error) {
	orgId := uc.enforceSecurity.OrgId()

	return executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) (models.ScoringDryRun, error) {
		draft, err := uc.repository.GetScoringRuleset(ctx, tx, orgId, recordType, models.ScoreRulesetDraft, 0)
		if err != nil {
			if errors.Is(err, models.NotFoundError) {
				return models.ScoringDryRun{}, errors.Wrap(err, "no draft version found")
			}

			return models.ScoringDryRun{}, err
		}

		if err := uc.enforceSecurity.UpdateRuleset(orgId); err != nil {
			return models.ScoringDryRun{}, err
		}

		indexes, pending, err := uc.indexEditor.GetIndexesToCreateForScoringRuleset(ctx, orgId, draft)
		if err != nil {
			return models.ScoringDryRun{}, err
		}

		if pending > 0 {
			return models.ScoringDryRun{}, errors.Wrap(models.UnprocessableEntityError, "ruleset is still being prepared")
		}
		if len(indexes) > 0 {
			return models.ScoringDryRun{}, errors.Wrap(models.UnprocessableEntityError, "ruleset is not prepared")
		}

		dryRun, err := uc.repository.InsertRulesetDryRun(ctx, tx, draft, RULESET_DRY_RUN_SAMPLE_SIZE)
		if err != nil {
			return models.ScoringDryRun{}, err
		}

		if err := uc.taskQueueRepository.EnqueueRulesetDryRun(ctx, tx, orgId, dryRun); err != nil {
			return models.ScoringDryRun{}, err
		}

		return dryRun, nil
	})
}

func (uc ScoringRulesetsUsecase) CommitRuleset(ctx context.Context, recordType string) (models.ScoringRuleset, error) {
	orgId := uc.enforceSecurity.OrgId()

	if err := uc.isScoringEnabled(ctx, orgId); err != nil {
		return models.ScoringRuleset{}, err
	}

	if err := uc.enforceSecurity.UpdateRuleset(orgId); err != nil {
		return models.ScoringRuleset{}, err
	}

	ruleset, err := executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) (models.ScoringRuleset, error) {
		draft, err := uc.repository.GetScoringRuleset(ctx, tx, orgId, recordType, models.ScoreRulesetDraft, 0)
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

		if err := uc.repository.CancelRulesetDryRun(ctx, tx, draft); err != nil {
			return models.ScoringRuleset{}, err
		}

		return uc.repository.CommitRuleset(ctx, tx, draft)
	})

	if err == nil {
		cache := uc.redisClient.NewExecutor(orgId)
		_ = cache.Exec(func(c *redis.Client) error {
			return c.Set(ctx, cache.Key("scoring_record_types", recordType), true, time.Hour).Err()
		})
	}

	return ruleset, err
}

func (uc ScoringRulesetsUsecase) ValidateAst(ctx context.Context, recordType string, node *ast.Node) (models.AstValidation, error) {
	scenario := models.Scenario{
		OrganizationId:    uc.enforceSecurity.OrgId(),
		TriggerObjectType: recordType,
	}

	validation := uc.validateScenarioAst.Validate(ctx, scenario, node, "score_computation_result")

	return validation, nil
}

func (uc ScoringRulesetsUsecase) validateScoringRuleAst(tree dto.NodeDto) error {
	if tree.Name == ast.FuncAttributesMap[ast.FUNC_SWITCH].AstName {
		for _, child := range tree.Children {
			if child.Name != ast.FuncAttributesMap[ast.FUNC_SCORE_COMPUTATION].AstName {
				return errors.New("invalid root AST node for user scoring: all `Switch` children must be `ScoreComputation`")
			}
		}
	}

	return nil
}

func (uc ScoringRulesetsUsecase) isScoringEnabled(ctx context.Context, orgId uuid.UUID) error {
	featureAccess, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, orgId, nil)
	if err != nil {
		return err
	}
	if !featureAccess.UserScoring.IsAllowed() {
		return errors.Wrap(models.ForbiddenError, "cannot access user scoring feature")
	}
	return nil
}
