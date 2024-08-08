package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func selectSnoozeGroups() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectSnoozeGroupsColumn...).
		From(dbmodels.TABLE_SNOOZE_GROUPS)
}

func (repo *MarbleDbRepository) CreateSnoozeGroup(ctx context.Context, exec Executor, id, organizationId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_SNOOZE_GROUPS).
			Columns(
				"id",
				"organization_id",
				"created_at",
			).
			Values(
				id,
				organizationId,
				"NOW()",
			),
	)
	return err
}

func (repo *MarbleDbRepository) GetSnoozeGroup(ctx context.Context, exec Executor, id string) (models.SnoozeGroup, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SnoozeGroup{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectSnoozeGroups().Where(squirrel.Eq{"id": id}),
		dbmodels.AdaptSnoozeGroup,
	)
}

func (repo *MarbleDbRepository) CreateRuleSnooze(ctx context.Context, exec Executor, input models.RuleSnoozeCreateInput) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_RULE_SNOOZES).
			Columns(
				"id",
				"created_at",
				"created_by_user",
				"created_from_decision_id",
				"snooze_group_id",
				"pivot_value",
				"starts_at",
				"expires_at",
			).
			Values(
				input.Id,
				"NOW()",
				input.CreatedByUserId,
				input.CreatedFromDecisionId,
				input.SnoozeGroupId,
				input.PivotValue,
				"NOW()",
				input.ExpiresAt,
			),
	)
	return err
}

func (repo *MarbleDbRepository) GetRuleSnooze(ctx context.Context, exec Executor, id string) (models.RuleSnooze, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.RuleSnooze{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectRuleSnoozesColumn...).
			From(dbmodels.TABLE_RULE_SNOOZES).
			Where(squirrel.Eq{"id": id}),
		dbmodels.AdaptRuleSnooze,
	)
}

func (repo *MarbleDbRepository) ListRuleSnoozesForDecision(
	ctx context.Context,
	exec Executor,
	snoozeGroupIds []string,
	pivotValue string,
) ([]models.RuleSnooze, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectRuleSnoozesColumn...).
			From(dbmodels.TABLE_RULE_SNOOZES).
			Where(squirrel.Eq{"snooze_group_id": snoozeGroupIds, "pivot_value": pivotValue}).
			Limit(200),
		dbmodels.AdaptRuleSnooze,
	)
}

func (repo *MarbleDbRepository) AnySnoozesForIteration(
	ctx context.Context,
	exec Executor,
	snoozeGroupIds []string,
) (map[string]bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	m := make(map[string]bool)
	query := `
	SELECT snooze_group_id, COUNT(*) > 0 AS has_any
	FROM rule_snoozes
	WHERE snooze_group_id = ANY($1)
	AND expires_at > NOW()
	GROUP BY snooze_group_id`

	rows, err := exec.Query(ctx, query, snoozeGroupIds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var snoozeGroupId string
		var hasAny bool
		err := rows.Scan(&snoozeGroupId, &hasAny)
		if err != nil {
			return nil, err
		}
		m[snoozeGroupId] = hasAny
	}

	return m, rows.Err()
}
