package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_GRANTS = "grants"

const (
	GrantPrincipalUser   = "user"
	GrantPrincipalApiKey = "api_key"
	GrantAuthorityMarble = "marble"
)

type DbRoleBinding struct {
	Id             uuid.UUID  `db:"id"`
	PrincipalType  string     `db:"principal_type"`
	PrincipalId    string     `db:"principal_id"`
	TenantId       *uuid.UUID `db:"tenant_id"`
	OrganizationId *uuid.UUID `db:"organization_id"`
	Role           string     `db:"role"`
}

var SelectRoleBindingColumns = utils.ColumnList[DbRoleBinding]("g")

func AdaptRoleBinding(db DbRoleBinding) (models.RoleBinding, error) {
	binding := models.RoleBinding{
		Id:          db.Id,
		Role:        models.Role(db.Role),
		Permissions: models.Role(db.Role).Permissions(),
	}

	if db.TenantId != nil {
		binding.TenantId = *db.TenantId
	}
	if db.OrganizationId != nil {
		binding.OrgId = *db.OrganizationId
	}

	switch db.PrincipalType {
	case GrantPrincipalUser:
		userId := models.UserId(db.PrincipalId)
		binding.UserId = &userId
	case GrantPrincipalApiKey:
		if apiKeyId, err := uuid.Parse(db.PrincipalId); err == nil {
			binding.ApiKeyId = &apiKeyId
		}
	}

	return binding, nil
}
