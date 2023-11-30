package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

type DBTag struct {
	Id             string           `db:"id"`
	Name           string           `db:"name"`
	Color          string           `db:"color"`
	OrganizationId string           `db:"org_id"`
	CreatedAt      time.Time        `db:"created_at"`
	UpdatedAt      time.Time        `db:"updated_at"`
	DeletedAt      pgtype.Timestamp `db:"deleted_at"`
}

const TABLE_TAGS = "tags"

var SelectTagColumn = utils.ColumnList[DBTag]()

func AdaptTag(db DBTag) (models.Tag, error) {
	return models.Tag{
		Id:             db.Id,
		Name:           db.Name,
		Color:          db.Color,
		OrganizationId: db.OrganizationId,
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
	}, nil
}
