package dbmodels

import (
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"
	"time"
)

type DBListResult struct {
	Id          string    `db:"id"`
	OrgId       string    `db:"org_id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

const TABLE_LIST = "lists"

var ColumnsSelectList = pg_repository.ColumnList[DBListResult]()

func AdaptList(db DBListResult) models.List {

	return models.List{
		Id:          db.Id,
		OrgId:       db.OrgId,
		Name:        db.Name,
		Description: db.Description,
		CreatedAt:   db.CreatedAt,
		UpdatedAt:   db.UpdatedAt,
	}
}