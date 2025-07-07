package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) GetWatermark(ctx context.Context, exec Executor, orgId *string,
	watermarkType models.WatermarkType,
) (*models.Watermark, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	whereClause := squirrel.And{squirrel.Eq{"type": watermarkType.String()}}
	if orgId != nil {
		whereClause = append(whereClause, squirrel.Eq{"org_id": *orgId})
	} else {
		whereClause = append(whereClause, squirrel.Eq{"org_id": nil})
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectWatermarkColumn...).
		From(dbmodels.TABLE_WATERMARKS).
		Where(whereClause)

	return SqlToOptionalModel(ctx, exec, sql, dbmodels.AdaptWatermark)
}

func (repo *MarbleDbRepository) SaveWatermark(ctx context.Context, tx Transaction,
	orgId *string, watermarkType models.WatermarkType, watermarkId *string, watermarkTime time.Time, params json.RawMessage,
) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_WATERMARKS).
		Columns("org_id", "type", "watermark_time", "watermark_id", "created_at", "updated_at", "params").
		Values(
			orgId,
			watermarkType.String(),
			watermarkTime,
			watermarkId,
			time.Now(),
			time.Now(),
			params,
		).
		Suffix("on conflict (org_id, type) do update set").
		Suffix("watermark_time = excluded.watermark_time,").
		Suffix("watermark_id = excluded.watermark_id,").
		Suffix("updated_at = excluded.updated_at,").
		Suffix("params = excluded.params")

	return ExecBuilder(ctx, tx, sql)
}
