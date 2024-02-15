package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBCustomListValueResult struct {
	Id           string     `db:"id"`
	CustomListId string     `db:"custom_list_id"`
	Value        string     `db:"value"`
	CreatedAt    time.Time  `db:"created_at"`
	DeletedAt    *time.Time `db:"deleted_at"`
}

const TABLE_CUSTOM_LIST_VALUE = "custom_list_values"

var ColumnsSelectCustomListValue = utils.ColumnList[DBCustomListValueResult]()

func AdaptCustomListValue(db DBCustomListValueResult) (models.CustomListValue, error) {
	return models.CustomListValue{
		Id:           db.Id,
		CustomListId: db.CustomListId,
		Value:        db.Value,
		CreatedAt:    db.CreatedAt,
		DeletedAt:    db.DeletedAt,
	}, nil
}
