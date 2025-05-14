package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DbOffloadingWatermark struct {
	OrgId         string    `db:"org_id"`
	TableName     string    `db:"table_name"`
	WatermarkTime time.Time `db:"watermark_time"`
	WatermarkId   string    `db:"watermark_id"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

const TABLE_OFFLOADING_WATERMARKS = "offloading_watermarks"

var SelectOffloadingWatermarkColumn = utils.ColumnList[DbOffloadingWatermark]()

func AdaptOffloadingWatermark(db DbOffloadingWatermark) (models.OffloadingWatermark, error) {
	return models.OffloadingWatermark{
		OrgId:         db.OrgId,
		TableName:     db.TableName,
		WatermarkTime: db.WatermarkTime,
		WatermarkId:   db.WatermarkId,
		CreatedAt:     db.CreatedAt,
		UpdatedAt:     db.UpdatedAt,
	}, nil
}
