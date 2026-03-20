package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) GetScoringLatestDryRun(
	ctx context.Context,
	exec Executor,
	rulesetId uuid.UUID,
) (models.ScoringDryRun, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScoringDryRun{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectScoringDryRunsColumns...).
		From(dbmodels.TABLE_SCORING_DRY_RUNS).
		Where("ruleset_id = ?", rulesetId).
		OrderBy("created_at desc").
		Limit(1)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptScoringDryRun)
}

func (repo *MarbleDbRepository) GetScoringDryRunById(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
) (models.ScoringDryRun, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScoringDryRun{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectScoringDryRunsColumns...).
		From(dbmodels.TABLE_SCORING_DRY_RUNS).
		Where("id = ?", id)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptScoringDryRun)
}

func (repo *MarbleDbRepository) InsertRulesetDryRun(
	ctx context.Context,
	tx Transaction,
	ruleset models.ScoringRuleset,
	objectCount int,
) (models.ScoringDryRun, error) {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return models.ScoringDryRun{}, err
	}

	cancelQuery := NewQueryBuilder().
		Update(dbmodels.TABLE_SCORING_DRY_RUNS).
		Set("status", models.DryRunCancelled).
		Where("status = any(?)", []models.DryRunStatus{models.DryRunPending, models.DryRunRunning}).
		Where("ruleset_id = ?", ruleset.Id)

	if err := ExecBuilder(ctx, tx, cancelQuery); err != nil {
		return models.ScoringDryRun{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCORING_DRY_RUNS).
		Columns("id", "ruleset_id", "record_count").
		Values(
			pure_utils.NewId(),
			ruleset.Id,
			objectCount,
		).
		Suffix("returning *")

	return SqlToModel(ctx, tx, query, dbmodels.AdaptScoringDryRun)
}

func (repo *MarbleDbRepository) CancelRulesetDryRun(
	ctx context.Context,
	exec Executor,
	ruleset models.ScoringRuleset,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SCORING_DRY_RUNS).
		Set("status", models.DryRunCancelled).
		Where("status = any(?)", []models.DryRunStatus{models.DryRunPending, models.DryRunRunning}).
		Where("ruleset_id = ?", ruleset.Id)

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) SetRulesetDryRunStatus(
	ctx context.Context,
	exec Executor,
	dryRun models.ScoringDryRun,
	status models.DryRunStatus,
	results map[int]int,
) (*models.ScoringDryRun, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SCORING_DRY_RUNS).
		Set("status", status).
		Set("results", results).
		Where("id = ?", dryRun.Id).
		Where("status != ?", models.DryRunCancelled).
		Suffix("returning *")

	return SqlToOptionalModel(ctx, exec, query, dbmodels.AdaptScoringDryRun)
}
