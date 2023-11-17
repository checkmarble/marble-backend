package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBUserResult struct {
	Id             string  `db:"id"`
	Email          string  `db:"email"`
	FirebaseUid    string  `db:"firebase_uid"`
	Role           int     `db:"role"`
	OrganizationId *string `db:"organization_id"`
	FirstName      *string  `db:"first_name"`
	LastName       *string  `db:"last_name"`
}

const TABLE_USERS = "users"

var UserFields = utils.ColumnList[DBUserResult]()

func AdaptUser(db DBUserResult) (models.User, error) {
	var organizationId, firstName, lastName string
	if db.OrganizationId != nil {
		organizationId = *db.OrganizationId
	}
	if db.FirstName != nil {
		firstName = *db.FirstName
	}
	if db.LastName != nil {
		lastName = *db.LastName
	}
	return models.User{
		UserId:         models.UserId(db.Id),
		Email:          db.Email,
		FirebaseUid:    db.FirebaseUid,
		Role:           models.Role(db.Role),
		OrganizationId: organizationId,
		FirstName:      firstName,
		LastName:       lastName,
	}, nil
}
