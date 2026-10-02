package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbRole struct {
	Id          uuid.UUID `db:"id"`
	OrgId       uuid.UUID `db:"org_id"`
	Slug        string    `db:"slug"`
	Name        string    `db:"name"`
	Permissions []string  `db:"permissions"`
}

const (
	TABLE_ROLES = "roles"
)

var SelectRoleColumn = utils.ColumnList[DbRole]()

func AdaptRole(db DbRole) (models.RbacRole, error) {
	return models.RbacRole{
		Id:    db.Id,
		OrgId: db.OrgId,
		Slug:  db.Slug,
		Name:  db.Name,
		Permissions: models.WithoutPlatformPermissions(
			pure_utils.Map(db.Permissions, func(permission string) models.Permission {
				return models.Permission(permission)
			})),
	}, nil
}
