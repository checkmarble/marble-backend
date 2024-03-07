package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

type DBUserResult struct {
	Id             string             `db:"id"`
	Email          string             `db:"email"`
	Role           int                `db:"role"`
	OrganizationId *string            `db:"organization_id"`
	FirstName      pgtype.Text        `db:"first_name"`
	LastName       pgtype.Text        `db:"last_name"`
	DeletedAt      pgtype.Timestamptz `db:"deleted_at"`
}

const TABLE_USERS = "users"

var UserFields = utils.ColumnList[DBUserResult]()

func AdaptUser(db DBUserResult) (models.User, error) {
	user := models.User{
		UserId: models.UserId(db.Id),
		Email:  db.Email,
		Role:   models.Role(db.Role),
	}
	if db.OrganizationId != nil {
		user.OrganizationId = *db.OrganizationId
	}
	if db.FirstName.Valid {
		user.FirstName = db.FirstName.String
	}
	if db.LastName.Valid {
		user.LastName = db.LastName.String
	}
	if db.DeletedAt.Valid {
		user.DeletedAt = &db.DeletedAt.Time
	}
	return user, nil
}
