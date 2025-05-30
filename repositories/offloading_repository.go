package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

const (
	OffloadingDecisionRules = "decision_rules"
)

func (repo *MarbleDbRepository) GetOffloadedDecisionRuleKey(
	orgId, decisionId, ruleId, outcome string, createdAt time.Time,
) string {
	if outcome == "" {
		outcome = "no_hit"
	}

	return fmt.Sprintf("offloading/decision_rules/%s/%s/%d/%d/%s/%s", outcome, orgId,
		createdAt.Year(), createdAt.Month(), decisionId, ruleId)
}

func (repo *MarbleDbRepository) GetOffloadingWatermark(ctx context.Context, exec Executor, orgId, table string) (*models.OffloadingWatermark, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectOffloadingWatermarkColumn...).
		From(dbmodels.TABLE_OFFLOADING_WATERMARKS).
		Where(squirrel.And{
			squirrel.Eq{"org_id": orgId},
			squirrel.Eq{"table_name": table},
		})

	return SqlToOptionalModel(ctx, exec, sql, dbmodels.AdaptOffloadingWatermark)
}

func (repo *MarbleDbRepository) SaveOffloadingWatermark(ctx context.Context, tx Transaction,
	orgId, table, watermarkId string, watermarkTime time.Time,
) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_OFFLOADING_WATERMARKS).
		Columns("org_id", "table_name", "watermark_time", "watermark_id", "created_at", "updated_at").
		Values(
			orgId,
			table,
			watermarkTime,
			watermarkId,
			time.Now(),
			time.Now(),
		).
		Suffix("on conflict (org_id, table_name) do update set").
		Suffix("watermark_time = excluded.watermark_time,").
		Suffix("watermark_id = excluded.watermark_id,").
		Suffix("updated_at = excluded.updated_at")

	return ExecBuilder(ctx, tx, sql)
}
