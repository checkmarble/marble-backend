package repositories

import (
	"context"
	"fmt"
	"strings"

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
		Options("distinct on (record_type)").
		From(dbmodels.TABLE_SCORING_RULESETS).
		Where("org_id = ?", orgId).
		OrderBy("record_type", "version desc")

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
	recordType string,
	status models.ScoreRulesetStatus,
	version int,
) (models.ScoringRuleset, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScoringRuleset{}, err
	}

	cte := WithCtes("ruleset", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
		q := b.
			Select(dbmodels.SelectScoringRulesetsColumns...).
			From(dbmodels.TABLE_SCORING_RULESETS).
			Where("org_id = ?", orgId).
			Where("record_type = ?", recordType).
			OrderBy("version desc").
			Limit(1)

		if version > 0 {
			q = q.Where("version = ?", version)
		} else {
			q = q.Where("status = ?", status)
		}

		return q
	})

	query := NewQueryBuilder().
		Select(
			fmt.Sprintf("any_value(row(%s)) as ruleset",
				strings.Join(columnsNames("rs", dbmodels.SelectScoringRulesetsColumns), ",")),
			fmt.Sprintf("array_agg(row(%s)) filter (where r.id is not null) as rules",
				strings.Join(columnsNames("r", dbmodels.SelectScoringRulesColumns), ",")),
		).
		From("ruleset rs").
		LeftJoin(dbmodels.TABLE_SCORING_RULES + " r on r.ruleset_id = rs.id").
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

func (repo *MarbleDbRepository) ListScoringRulesetVersions(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	recordType string,
) ([]models.ScoringRuleset, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectScoringRulesetsColumns...).
		From(dbmodels.TABLE_SCORING_RULESETS).
		Where("org_id = ?", orgId).
		Where("record_type = ?", recordType).
		OrderBy("version desc")

	rulesets, err := SqlToListOfModels(ctx, exec, query, dbmodels.AdaptScoringRuleset)
	if err != nil {
		return nil, err
	}

	return rulesets, nil
}

func (repo *MarbleDbRepository) GetScoringRulesetById(
	ctx context.Context,
	exec Executor,
	orgId, id uuid.UUID,
) (models.ScoringRuleset, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScoringRuleset{}, err
	}

	cte := WithCtes("ruleset", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
		return b.
			Select(dbmodels.SelectScoringRulesetsColumns...).
			From(dbmodels.TABLE_SCORING_RULESETS).
			Where("org_id = ?", orgId).
			Where("id = ?", id).
			Limit(1)
	})

	query := NewQueryBuilder().
		Select(
			fmt.Sprintf("any_value(row(%s)) as ruleset",
				strings.Join(columnsNames("rs", dbmodels.SelectScoringRulesetsColumns), ",")),
			fmt.Sprintf("array_agg(row(%s)) filter (where r.id is not null) as rules",
				strings.Join(columnsNames("r", dbmodels.SelectScoringRulesColumns), ",")),
		).
		From("ruleset rs").
		LeftJoin(dbmodels.TABLE_SCORING_RULES + " r on r.ruleset_id = rs.id").
		GroupBy("rs.id").
		PrefixExpr(cte)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptScoringRulesetAndRules)
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
			"record_type",
			"thresholds",
			"cooldown_seconds",
			"scoring_interval_seconds",
		).
		Values(
			uuid.Must(uuid.NewV7()),
			orgId,
			ruleset.Version,
			ruleset.Name,
			ruleset.Description,
			ruleset.RecordType,
			ruleset.Thresholds,
			ruleset.CooldownSeconds,
			ruleset.ScoringIntervalSeconds,
		).
		Suffix(`
			on conflict (org_id, record_type) where status = 'draft'
			do update set
				name = excluded.name,
				description = excluded.description,
				thresholds = excluded.thresholds,
				cooldown_seconds = excluded.cooldown_seconds,
				scoring_interval_seconds = excluded.scoring_interval_seconds
		`).
		Suffix(fmt.Sprintf("returning %s", strings.Join(dbmodels.SelectScoringRulesetsColumns, ",")))

	return SqlToModel(ctx, tx, query, dbmodels.AdaptScoringRuleset)
}

func (repo *MarbleDbRepository) InsertScoringRulesetVersionRule(
	ctx context.Context,
	tx Transaction,
	ruleset models.ScoringRuleset,
	rules []models.CreateScoringRuleRequest,
) ([]models.ScoringRule, error) {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return nil, err
	}

	deleteQuery := NewQueryBuilder().
		Delete(dbmodels.TABLE_SCORING_RULES).
		Where("ruleset_id = ?", ruleset.Id)

	if err := ExecBuilder(ctx, tx, deleteQuery); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCORING_RULES).
		Columns(
			"id",
			"ruleset_id",
			"stable_id",
			"name",
			"description",
			"risk_type",
			"ast",
		).
		Suffix("returning *")

	for _, rule := range rules {
		query = query.Values(
			uuid.Must(uuid.NewV7()),
			ruleset.Id,
			rule.StableId,
			rule.Name,
			rule.Description,
			rule.RiskType,
			rule.Ast,
		)
	}

	return SqlToListOfModels(ctx, tx, query, dbmodels.AdaptScoringRule)
}

func (repo *MarbleDbRepository) CommitRuleset(ctx context.Context, exec Executor, ruleset models.ScoringRuleset) (models.ScoringRuleset, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScoringRuleset{}, err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SCORING_RULESETS).
		Set("status", models.ScoreRulesetCommitted).
		Where("org_id = ?", ruleset.OrgId).
		Where("id = ?", ruleset.Id).
		Suffix("returning *")

	return SqlToModel(ctx, exec, query, dbmodels.AdaptScoringRuleset)
}
