package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityTenant interface {
	EnforceSecurity
	UpdateTenant(tenantId uuid.UUID) error
}

type EnforceSecurityTenantImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityTenantImpl) UpdateTenant(tenantId uuid.UUID) error {
	if err := e.Permission(models.TENANTS_UPDATE); err != nil {
		return err
	}
	if tenantId == uuid.Nil {
		return errors.Wrap(models.ForbiddenError, "tenant ID is required")
	}
	if e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return nil
	}
	if e.Credentials.TenantId == uuid.Nil || e.Credentials.TenantId != tenantId {
		return errors.Wrap(models.ForbiddenError, "credentials do not grant access to tenant")
	}
	return nil
}
