package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_GRANTS = "grants"

var TenantGrantColumns = utils.ColumnList[DbTenantGrant]()

type DbTenantGrant struct {
	Id                 uuid.UUID `db:"id"`
	PrincipalType      string    `db:"principal_type"`
	PrincipalId        string    `db:"principal_id"`
	PrincipalAuthority string    `db:"principal_authority"`
	TenantId           uuid.UUID `db:"tenant_id"`
	Role               string    `db:"role"`
}

type DBOrganizationUserGrant struct {
	DBUserResult
	OrganizationGrantRole *string `db:"organization_grant_role"`
}

func AdaptOrganizationUserGrant(db DBOrganizationUserGrant) (models.OrganizationUserGrant, error) {
	user, err := AdaptUser(db.DBUserResult)
	if err != nil {
		return models.OrganizationUserGrant{}, err
	}
	grant := models.OrganizationUserGrant{User: user}
	if db.OrganizationGrantRole != nil {
		role := models.RoleFromString(*db.OrganizationGrantRole)
		grant.OrganizationGrantRole = &role
	}
	return grant, nil
}
