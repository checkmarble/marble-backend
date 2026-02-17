package utils

import (
	"context"
	"fmt"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

func CredentialsFromCtx(ctx context.Context) (models.Credentials, bool) {
	creds, ok := ctx.Value(ContextKeyCredentials).(models.Credentials)
	return creds, ok
}

func OrganizationIdFromRequest(request *http.Request) (organizationId uuid.UUID, err error) {
	if request == nil {
		return uuid.Nil, fmt.Errorf("no request passed to OrganizationIdFromRequest: %w", models.ForbiddenError)
	}

	creds, found := CredentialsFromCtx(request.Context())
	if !found {
		return uuid.Nil, fmt.Errorf("no credentials in context: %w", models.ForbiddenError)
	}

	// allow organizationId to be passed in query param
	requestOrganizationId := request.URL.Query().Get("organization-id")
	if requestOrganizationId != "" {
		if err := ValidateUuid(requestOrganizationId); err != nil {
			return uuid.Nil, err
		}

		requestOrgUUID, err := uuid.Parse(requestOrganizationId)
		if err != nil {
			return uuid.Nil, err
		}

		// technically, any user can pass an org id in query params, but it can be different from the credentials org id
		// only for a marble admin user
		if err := EnforceOrganizationAccess(creds, requestOrgUUID); err != nil {
			return uuid.Nil, err
		}
		return requestOrgUUID, nil
	}

	if creds.OrganizationId == uuid.Nil {
		if creds.Role == models.MARBLE_ADMIN {
			return uuid.Nil, errors.Wrap(
				models.ForbiddenError,
				"An organizationId must be passed in the request query params for MARBLE_ADMIN to use this endpoint")
		}
		return uuid.Nil, errors.Wrap(
			models.ForbiddenError,
			"Unexpected error: credentials does not grant access to any organization")
	}

	return creds.OrganizationId, nil
}
