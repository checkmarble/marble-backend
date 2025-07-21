package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbUserUnavailability struct {
	Id        uuid.UUID  `db:"id"`
	OrgId     uuid.UUID  `db:"org_id"`
	UserId    uuid.UUID  `db:"user_id"`
	FromDate  time.Time  `db:"from_date"`
	UntilDate time.Time  `db:"until_date"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

const TABLE_USER_UNAVAILABILITIES = "user_unavailabilities"

var ColumnsSelectUserUnavailabilities = utils.ColumnList[DbUserUnavailability]()

func AdaptUserUnavailability(db DbUserUnavailability) (models.UserUnavailability, error) {
	return models.UserUnavailability{
		Id:        db.Id,
		OrgId:     db.OrgId,
		UserId:    db.UserId,
		FromDate:  db.FromDate,
		UntilDate: db.UntilDate,
		CreatedAt: db.CreatedAt,
		UpdatedAt: db.UpdatedAt,
	}, nil
}
