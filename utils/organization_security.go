package utils

import (
	"fmt"
	"marble/marble-backend/models"
)

func EnforceOrganizationAccess(creds models.Credentials, organizationID string) error {

	noOrgIdSecurity := creds.Role.HasPermission(models.ANY_ORGANIZATION_ID_IN_CONTEXT)
	if noOrgIdSecurity {
		return nil
	}

	if organizationID == "" {
		return fmt.Errorf("no organization ID: %w", models.BadParameterError)
	}

	if creds.OrganizationId == "" {
		return fmt.Errorf("credentials does not grant access to any organization: %w", models.ForbiddenError)
	}

	if creds.OrganizationId != organizationID {
		return fmt.Errorf("credentials does not grant access to organization %s: %w", organizationID, models.ForbiddenError)
	}

	return nil
}
