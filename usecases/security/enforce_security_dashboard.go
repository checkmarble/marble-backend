package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

type EnforceSecurityDashboard interface {
	EnforceSecurity

	ReadDashboard() error
}

type EnforceSecurityDashboardImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

// The dashboard aggregates every organization, tenant and user of the platform.
func (e *EnforceSecurityDashboardImpl) ReadDashboard() error {
	if !e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return errors.Wrap(models.ForbiddenError, "only marble admins can read the dashboard")
	}

	return nil
}
