package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

type EnforceSecurityAudit interface {
	EnforceSecurity

	ReadAuditEvents() error
}

type EnforceSecurityAuditImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityAuditImpl) ReadAuditEvents() error {
	if e.Credentials.Role != models.ADMIN {
		return errors.Wrap(models.ForbiddenError, "only admins can read audit events")
	}

	return e.ReadOrganization(e.Credentials.OrganizationId)
}
