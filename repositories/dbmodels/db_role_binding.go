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
	CustomRoleFound      bool     `db:"custom_role_found"`
	CustomPermissions    []string `db:"custom_permissions"`
	CustomHiddenSections []string `db:"custom_hidden_sections"`
}

var SelectRoleBindingColumns = append(
	utils.ColumnList[DbGrant]("g"),
	"r.id is not null as custom_role_found",
	"coalesce(r.permissions, array[]::text[]) as custom_permissions",
	"coalesce(r.hidden_sections, array[]::text[]) as custom_hidden_sections",
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

	switch {
	case db.CustomRoleId == nil:
		binding.Permissions = binding.Role.Permissions()
		binding.HiddenSections = binding.Role.HiddenSections()
	case !db.CustomRoleFound:
		binding.HiddenSections = models.AppSections()
	default:
		binding.Permissions = models.WithoutPlatformPermissions(
			pure_utils.Map(db.CustomPermissions, func(permission string) models.Permission {
				return models.Permission(permission)
			}))
		binding.HiddenSections = pure_utils.Map(db.CustomHiddenSections, func(section string) models.AppSection {
			return models.AppSection(section)
		})
	}

	return binding, nil
}
