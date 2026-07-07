package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbRole struct {
	Id    uuid.UUID `db:"id"`
	OrgId uuid.UUID `db:"org_id"`
	Name  string    `db:"name"`
}

type DbPermission struct {
	Id        uuid.UUID `db:"id" json:"id"`
	OrgId     uuid.UUID `db:"org_id" json:"org_id"`
	RoleId    uuid.UUID `db:"role_id" json:"role_id"`
	Name      string    `db:"name" json:"name"`
	Condition *string   `db:"condition" json:"condition"`
}

type DbRoleWithPermissions struct {
	DbRole
	Permissions []DbPermission `db:"permissions"`
}

const (
	TABLE_ROLES       = "roles"
	TABLE_PERMISSIONS = "permissions"
)

var (
	SelectRoleColumn       = utils.ColumnList[DbRole]()
	SelectPermissionColumn = utils.ColumnList[DbPermission]()
)

func AdaptRole(db DbRole) (models.RbacRole, error) {
	return models.RbacRole{
		Id:    db.Id,
		OrgId: db.OrgId,
		Name:  db.Name,
	}, nil
}

func AdaptRoleWithPermissions(db DbRoleWithPermissions) (models.RbacRole, error) {
	role, err := AdaptRole(db.DbRole)
	if err != nil {
		return models.RbacRole{}, err
	}

	role.Permissions = pure_utils.Map(db.Permissions, func(p DbPermission) models.RbacPermission {
		return models.RbacPermission{
			Id:        p.Id,
			OrgId:     p.OrgId,
			Name:      p.Name,
			Condition: p.Condition,
		}
	})

	return role, nil
}
