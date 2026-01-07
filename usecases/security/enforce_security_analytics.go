package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type EnforceSecurityAnalyticsImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityAnalyticsImpl) ReadAnalytics(analytics models.Analytics) error {
	orgId, _ := uuid.Parse(analytics.OrganizationId) // Ignore error, will be uuid.Nil if invalid
	return errors.Join(
		e.Permission(models.ANALYTICS_READ), e.ReadOrganization(orgId),
	)
}
