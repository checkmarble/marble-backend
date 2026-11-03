package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_GRANTS = "grants"

const (
	GrantPrincipalUser   = "user"
	GrantPrincipalApiKey = "api_key"
	GrantAuthorityMarble = "marble"
)

type DbGrant struct {
	Id             uuid.UUID  `db:"id"`
	PrincipalType  string     `db:"principal_type"`
	PrincipalId    string     `db:"principal_id"`
	TenantId       *uuid.UUID `db:"tenant_id"`
	OrganizationId *uuid.UUID `db:"organization_id"`
	Role           string     `db:"role"`
	CustomRoleId   *uuid.UUID `db:"custom_role_id"`
}

type DbRoleBinding struct {
	DbGrant

	// Resolved from the custom role, if any.
	CustomPermissions []string `db:"custom_permissions"`
}

var SelectRoleBindingColumns = append(
	utils.ColumnList[DbGrant]("g"),
	"coalesce(r.permissions, array[]::text[]) as custom_permissions",
)

func AdaptRoleBinding(db DbRoleBinding) (models.RoleBinding, error) {
	binding := models.RoleBinding{
		Id:           db.Id,
		Role:         models.Role(db.Role),
		CustomRoleId: db.CustomRoleId,
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

	if db.CustomRoleId != nil {
		binding.Permissions = models.WithoutPlatformPermissions(
			pure_utils.Map(db.CustomPermissions, func(permission string) models.Permission {
				return models.Permission(permission)
			}))
	} else {
		binding.Permissions = binding.Role.Permissions()
	}

	return binding, nil
}
