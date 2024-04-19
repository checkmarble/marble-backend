package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

type DbPivot struct {
	Id             string      `db:"id"`
	BaseTableId    string      `db:"base_table_id"`
	CreatedAt      time.Time   `db:"created_at"`
	FieldId        pgtype.Text `db:"field_id"`
	OrganizationId string      `db:"organization_id"`
	PathLinkIds    []string    `db:"path_link_ids"`
}

const TABLE_DATA_MODEL_PIVOTS = "data_model_pivots"

var SelectPivotColumns = utils.ColumnList[DbPivot]()

func AdaptPivotMetadata(dbPivot DbPivot) (models.PivotMetadata, error) {
	pivot := models.PivotMetadata{
		Id:             dbPivot.Id,
		OrganizationId: dbPivot.OrganizationId,
		CreatedAt:      dbPivot.CreatedAt,

		BaseTableId: dbPivot.BaseTableId,
		PathLinkIds: dbPivot.PathLinkIds,
	}
	if dbPivot.FieldId.Valid {
		pivot.FieldId = &dbPivot.FieldId.String
	}

	return pivot, nil
}
