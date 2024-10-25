package utils

import (
	"context"
	"fmt"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

func CredentialsFromCtx(ctx context.Context) (models.Credentials, bool) {
	creds, ok := ctx.Value(ContextKeyCredentials).(models.Credentials)
	return creds, ok
}

func OrganizationIdFromRequest(request *http.Request) (organizationId string, err error) {
	if request == nil {
		return "", fmt.Errorf("no request passed to OrganizationIdFromRequest: %w", models.ForbiddenError)
	}

	creds, found := CredentialsFromCtx(request.Context())
	if !found {
		return "", fmt.Errorf("no credentials in context: %w", models.ForbiddenError)
	}

	// allow organizationId to be passed in query param
	requestOrganizationId := request.URL.Query().Get("organization-id")
	if requestOrganizationId != "" {
		if err := ValidateUuid(requestOrganizationId); err != nil {
			return "", err
		}

		// technically, any user can pass an org id in query params, but it can be different from the credentials org id
		// only for a marble admin user
		if err := EnforceOrganizationAccess(creds, requestOrganizationId); err != nil {
			return "", err
		}
		return requestOrganizationId, nil
	}

	if creds.OrganizationId == "" {
		if creds.Role == models.MARBLE_ADMIN {
			return "", errors.Wrap(
				models.ForbiddenError,
				"An organizationId must be passed in the request query params for MARBLE_ADMIN to use this endpoint")
		}
		return "", errors.Wrap(
			models.ForbiddenError,
			"Unexpected error: credentials does not grant access to any organization")
	}

	// if creds.OrganizationId == "" && creds.Role == models.MARBLE_ADMIN {
	// 	return "", errors.Wrap(
	// 		models.ForbiddenError,
	// 		"An organizationId must be passed in the request query params for MARBLE_ADMIN to use this endpoint")
	// }

	// if creds.OrganizationId == "" {
	// 	return "", errors.Wrap(
	// 		models.ForbiddenError,
	// 		"Unexpected error: credentials does not grant access to any organization")
	// }

	return creds.OrganizationId, nil
}
