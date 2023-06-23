package dbmodels

import (
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"
	"time"
)

type DBCustomListResult struct {
	Id          string    `db:"id"`
	OrgId       string    `db:"org_id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

const TABLE_CUSTOM_LIST = "custom_lists"

var ColumnsSelectCustomList = pg_repository.ColumnList[DBCustomListResult]()

func AdaptCustomList(db DBCustomListResult) models.CustomList {

	return models.CustomList{
		Id:          db.Id,
		OrgId:       db.OrgId,
		Name:        db.Name,
		Description: db.Description,
		CreatedAt:   db.CreatedAt,
		UpdatedAt:   db.UpdatedAt,
	}
}