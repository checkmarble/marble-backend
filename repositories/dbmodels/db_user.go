package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
)

type DBUserResult struct {
	Id             string  `db:"id"`
	Email          string  `db:"email"`
	FirebaseUid    string  `db:"firebase_uid"`
	Role           int     `db:"role"`
	OrganizationId *string `db:"organization_id"`
}

const TABLE_USERS = "users"

var UserFields = []string{"id", "email", "firebase_uid", "role", "organization_id"}

func AdaptUser(db DBUserResult) (models.User, error) {
	var organizationId string
	if db.OrganizationId != nil {
		organizationId = *db.OrganizationId
	}
	return models.User{
		UserId:         models.UserId(db.Id),
		Email:          db.Email,
		FirebaseUid:    db.FirebaseUid,
		Role:           models.Role(db.Role),
		OrganizationId: organizationId,
	}, nil
}
