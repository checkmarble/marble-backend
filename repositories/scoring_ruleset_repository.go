package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) ListScoringRulesets(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
) ([]models.ScoringRuleset, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectScoringRulesetsColumns...).
		Options("distinct on (entity_type)").
		From(dbmodels.TABLE_SCORING_RULESETS).
		Where("org_id = ?", orgId).
		OrderBy("entity_type", "version desc")

	rulesets, err := SqlToListOfModels(ctx, exec, query, dbmodels.AdaptScoringRuleset)
	if err != nil {
		return nil, err
	}

	return rulesets, nil
}

func (repo *MarbleDbRepository) GetScoringRuleset(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	entityType string,
) (models.ScoringRuleset, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScoringRuleset{}, err
	}

	cte := WithCtes("ruleset", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
		return b.
			Select(dbmodels.SelectScoringRulesetsColumns...).
			From(dbmodels.TABLE_SCORING_RULESETS).
			Where("org_id = ?", orgId).
			Where("entity_type = ?", entityType).
			OrderBy("version desc").
			Limit(1)
	})

	query := NewQueryBuilder().
		Select("any_value(rs.*) as ruleset", "array_agg(row(r.*)) as rules").
		From("ruleset rs").
		Join(dbmodels.TABLE_SCORING_RULES + " r on r.ruleset_id = rs.id").
		GroupBy("rs.id").
		PrefixExpr(cte)

	ruleset, err := SqlToOptionalModel(ctx, exec, query, dbmodels.AdaptScoringRulesetAndRules)
	if err != nil {
		return models.ScoringRuleset{}, err
	}

	if ruleset == nil {
		return models.ScoringRuleset{}, models.NotFoundError
	}

	return *ruleset, nil
}

func (repo *MarbleDbRepository) InsertScoringRulesetVersion(
	ctx context.Context,
	tx Transaction,
	orgId uuid.UUID,
	ruleset models.CreateScoringRulesetRequest,
) (models.ScoringRuleset, error) {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return models.ScoringRuleset{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCORING_RULESETS).
		Columns(
			"id",
			"org_id",
			"version",
			"name",
			"description",
			"entity_type",
			"thresholds",
			"cooldown_seconds",
		).
		Values(
			uuid.Must(uuid.NewV7()),
			orgId,
			ruleset.Version,
			ruleset.Name,
			ruleset.Description,
			ruleset.EntityType,
			ruleset.Thresholds,
			ruleset.CooldownSeconds,
		).
		Suffix("returning *")

	return SqlToModel(ctx, tx, query, dbmodels.AdaptScoringRuleset)
}

func (repo *MarbleDbRepository) InsertScoringRulesetVersionRule(
	ctx context.Context,
	tx Transaction,
	ruleset models.ScoringRuleset,
	rule models.CreateScoringRuleRequest,
) (models.ScoringRule, error) {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return models.ScoringRule{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCORING_RULES).
		Columns(
			"id",
			"ruleset_id",
			"stable_id",
			"name",
			"description",
			"ast",
		).
		Values(
			uuid.Must(uuid.NewV7()),
			ruleset.Id,
			rule.StableId,
			rule.Name,
			rule.Description,
			rule.Ast,
		).
		Suffix("returning *")

	return SqlToModel(ctx, tx, query, dbmodels.AdaptScoringRule)
}
