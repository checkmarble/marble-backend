package security

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type tenantPermissionMock struct {
	EnforceSecurity
	err error
}

func (m tenantPermissionMock) Permission(models.Permission) error { return m.err }

func TestUpdateTenant(t *testing.T) {
	tenant := uuid.New()
	other := uuid.New()
	for _, tt := range []struct {
		name   string
		creds  models.Credentials
		target uuid.UUID
		err    error
		allow  bool
	}{
		{name: "permission denied", creds: models.Credentials{Roles: []models.Role{models.MARBLE_ADMIN}}, target: tenant, err: models.ForbiddenError},
		{name: "global admin", creds: models.Credentials{Roles: []models.Role{models.MARBLE_ADMIN}}, target: tenant, allow: true},
		{name: "matching tenant", creds: models.Credentials{TenantId: tenant}, target: tenant, allow: true},
		{name: "different tenant", creds: models.Credentials{TenantId: other}, target: tenant},
		{name: "no tenant", target: tenant},
		{name: "empty target", creds: models.Credentials{Roles: []models.Role{models.MARBLE_ADMIN}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := EnforceSecurityTenantImpl{EnforceSecurity: tenantPermissionMock{err: tt.err}, Credentials: tt.creds}
			err := enforcer.UpdateTenant(tt.target)
			if tt.allow {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, models.ForbiddenError)
			}
		})
	}
}

func TestUpdateTenantOIDC(t *testing.T) {
	tenant := uuid.New()
	for _, tt := range []struct {
		name   string
		creds  models.Credentials
		target uuid.UUID
		allow  bool
	}{
		{name: "tenant admin", creds: models.Credentials{TenantId: tenant, Roles: []models.Role{models.TENANT_ADMIN}}, target: tenant, allow: true},
		{name: "other tenant", creds: models.Credentials{TenantId: uuid.New(), Roles: []models.Role{models.TENANT_ADMIN}}, target: tenant},
		{name: "organization admin", creds: models.Credentials{TenantId: tenant, Roles: []models.Role{models.ADMIN}}, target: tenant},
		{name: "OIDC API key", creds: models.Credentials{TenantId: tenant, Roles: []models.Role{models.OIDC_CONFIG_EDITOR}, ActorIdentity: models.Identity{ApiKeyId: "key"}}, target: tenant, allow: true},
		{name: "OIDC API key for another tenant", creds: models.Credentials{TenantId: uuid.New(), Roles: []models.Role{models.OIDC_CONFIG_EDITOR}, ActorIdentity: models.Identity{ApiKeyId: "key"}}, target: tenant},
		{name: "standard API key", creds: models.Credentials{TenantId: tenant, Roles: []models.Role{models.API_CLIENT}, ActorIdentity: models.Identity{ApiKeyId: "key"}}, target: tenant},
		{name: "marble admin", creds: models.Credentials{Roles: []models.Role{models.MARBLE_ADMIN}}, target: tenant, allow: true},
		{name: "empty tenant", creds: models.Credentials{Roles: []models.Role{models.MARBLE_ADMIN}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := EnforceSecurityTenantImpl{EnforceSecurity: NewEnforceSecurity(tt.creds), Credentials: tt.creds}
			err := enforcer.UpdateTenantOIDC(tt.target)
			if tt.allow {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, models.ForbiddenError)
			}
		})
	}
	creds := models.Credentials{TenantId: tenant, Roles: []models.Role{models.TENANT_ADMIN}}
	enforcer := EnforceSecurityTenantImpl{EnforceSecurity: NewEnforceSecurity(creds), Credentials: creds}
	require.ErrorIs(t, enforcer.UpdateTenant(tenant), models.ForbiddenError)
}
