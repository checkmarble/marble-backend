package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DbDataModelOptions struct {
	Id              string   `db:"id"`
	TableId         string   `db:"table_id"`
	DisplayedFields []string `db:"displayed_fields"`
}

const TABLE_DATA_MODEL_OPTIONS = "data_model_options"

var SelectDataModelOptionsColumns = utils.ColumnList[DbDataModelOptions]()

func AdaptDataModelOptions(db DbDataModelOptions) (models.DataModelOptions, error) {
	return models.DataModelOptions{
		Id:              db.Id,
		TableId:         db.TableId,
		DisplayedFields: db.DisplayedFields,
	}, nil
}
