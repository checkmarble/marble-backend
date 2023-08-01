package utils

import (
	"fmt"
	"marble/marble-backend/models"
)

func EnforceOrganizationAccess(creds models.Credentials, organizationId string) error {

	noOrgIdSecurity := creds.Role.HasPermission(models.ANY_ORGANIZATION_ID_IN_CONTEXT)
	if noOrgIdSecurity {
		return nil
	}

	if organizationId == "" {
		return fmt.Errorf("no organization Id: %w", models.BadParameterError)
	}

	if creds.OrganizationId == "" {
		return fmt.Errorf("credentials does not grant access to any organization: %w", models.ForbiddenError)
	}

	if creds.OrganizationId != organizationId {
		return fmt.Errorf("credentials does not grant access to organization %s: %w", organizationId, models.ForbiddenError)
	}

	return nil
}
