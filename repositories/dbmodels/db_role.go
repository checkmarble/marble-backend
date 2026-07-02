package dbmodels

import (
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbRole struct {
	Id          uuid.UUID `db:"id"`
	OrgId       uuid.UUID `db:"org_id"`
	Name        string    `db:"name"`
	Permissions []string  `db:"permissions"`
}

const TABLE_ROLES = "roles"

var SelectRoleColumn = utils.ColumnList[DbRole]()

func AdaptRolePermissions(role DbRole) ([]string, error) {
	return role.Permissions, nil
}

func AdaptRoleName(role DbRole) (string, error) {
	return role.Name, nil
}
