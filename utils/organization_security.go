package utils

import (
	"github.com/checkmarble/marble-backend/models"

	"github.com/cockroachdb/errors"
)

func EnforceOrganizationAccess(creds models.Credentials, organizationId string) error {
	noOrgIdSecurity := creds.Role.HasPermission(models.ANY_ORGANIZATION_ID_IN_CONTEXT)
	if noOrgIdSecurity {
		return nil
	}

	if organizationId == "" {
		return errors.New("Empty organization Id passed to EnforceOrganizationAccess")
	}

	if creds.OrganizationId == "" {
		return errors.Wrap(models.ForbiddenError, "credentials does not grant access to any organization")
	}

	if creds.OrganizationId != organizationId {
		return errors.Wrapf(models.ForbiddenError, "credentials does not grant access to organization %s", organizationId)
	}
	return nil
}

func EnforcePartnerAccess(creds models.Credentials, partnerId string) error {
	if partnerId == "" {
		return errors.Wrap(models.ForbiddenError, "API key with a valid partner_id is required")
	}
	if creds.PartnerId == nil || *creds.PartnerId != partnerId {
		return errors.Wrapf(models.ForbiddenError, "credentials does not grant access to partner %s", partnerId)
	}
	return nil
}
