package security

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOIDCRecoveryKeyAccess(t *testing.T) {
	org, other := uuid.New(), uuid.New()
	for _, tt := range []struct {
		name      string
		creds     models.Credentials
		target    uuid.UUID
		canCreate bool
		canManage bool
	}{
		{name: "organization admin", creds: models.Credentials{OrganizationId: org, Roles: []models.Role{models.ADMIN}}, target: org, canManage: true},
		{name: "tenant admin only", creds: models.Credentials{OrganizationId: org, Roles: []models.Role{models.TENANT_ADMIN}}, target: org},
		{name: "tenant and organization admin", creds: models.Credentials{OrganizationId: org, Roles: []models.Role{models.ADMIN, models.TENANT_ADMIN}}, target: org, canCreate: true, canManage: true},
		{name: "another organization", creds: models.Credentials{OrganizationId: org, Roles: []models.Role{models.ADMIN, models.TENANT_ADMIN}}, target: other},
		{name: "recovery key", creds: models.Credentials{OrganizationId: org, Roles: []models.Role{models.OIDC_CONFIG_EDITOR}}, target: org},
		{name: "marble admin", creds: models.Credentials{Roles: []models.Role{models.MARBLE_ADMIN}}, target: org, canCreate: true, canManage: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			guard := EnforceSecurityApiKeyImpl{EnforceSecurity: NewEnforceSecurity(tt.creds), Credentials: tt.creds}
			key := models.ApiKey{OrganizationId: tt.target, Role: models.OIDC_CONFIG_EDITOR}
			if tt.canCreate {
				require.NoError(t, guard.CreateApiKey(tt.target, key.Role))
			} else {
				require.ErrorIs(t, guard.CreateApiKey(tt.target, key.Role), models.ForbiddenError)
			}
			for _, err := range []error{guard.ReadApiKey(key), guard.DeleteApiKey(key)} {
				if tt.canManage {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, models.ForbiddenError)
				}
			}
		})
	}
}

func TestCheckApiKeyTenant(t *testing.T) {
	tenant := uuid.New()
	for _, tt := range []struct {
		name  string
		creds models.Credentials
		allow bool
	}{
		{name: "matching tenant", creds: models.Credentials{TenantId: tenant}, allow: true},
		{name: "different tenant", creds: models.Credentials{TenantId: uuid.New()}},
		{name: "missing tenant"},
		{name: "marble admin", creds: models.Credentials{Roles: []models.Role{models.MARBLE_ADMIN}}, allow: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := (&EnforceSecurityApiKeyImpl{Credentials: tt.creds}).CheckApiKeyTenant(tenant)
			if tt.allow {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, models.ForbiddenError)
			}
		})
	}
}
