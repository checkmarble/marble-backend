package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbWatermark struct {
	Id            uuid.UUID       `db:"id"`
	OrgId         *string         `db:"org_id"`
	Type          string          `db:"type"`
	WatermarkTime time.Time       `db:"watermark_time"`
	WatermarkId   *string         `db:"watermark_id"`
	CreatedAt     time.Time       `db:"created_at"` //nolint:tagliatelle
	UpdatedAt     time.Time       `db:"updated_at"`
	Params        json.RawMessage `db:"params"`
}

const TABLE_WATERMARKS = "watermarks"

var SelectWatermarkColumn = utils.ColumnList[DbWatermark]()

func AdaptWatermark(db DbWatermark) (models.Watermark, error) {
	watermarkType, err := models.WatermarkTypeFromString(db.Type)
	if err != nil {
		return models.Watermark{}, err
	}

	return models.Watermark{
		Id:            db.Id,
		OrgId:         db.OrgId,
		Type:          watermarkType,
		WatermarkTime: db.WatermarkTime,
		WatermarkId:   db.WatermarkId,
		CreatedAt:     db.CreatedAt,
		UpdatedAt:     db.UpdatedAt,
		Params:        db.Params,
	}, nil
}
