package dbmodels

import (
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"
	"time"
)

type DBListValueResult struct {
	Id        string    `db:"id"`
	ListId    string    `db:"list_id"`
	Value     string    `db:"value"`
	CreatedAt time.Time `db:"created_at"`
	DeletedAt time.Time `db:"deleted_at"`
}

const TABLE_LIST_VALUE = "list_value"

var ColumnsSelectListValue = pg_repository.ColumnList[DBListValueResult]()

func AdaptListValue(db DBListValueResult) models.ListValue {

	return models.ListValue{
		Id:          db.Id,
		ListId:       db.ListId,
		Value:        db.Value,
		CreatedAt:   db.CreatedAt,
		DeletedAt:   db.DeletedAt,
	}
}
