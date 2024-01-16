package utils

import (
	"context"
	"fmt"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
)

func CredentialsFromCtx(ctx context.Context) (models.Credentials, bool) {
	creds, ok := ctx.Value(ContextKeyCredentials).(models.Credentials)
	return creds, ok
}

func OrganizationIdFromRequest(request *http.Request) (organizationId string, err error) {
	creds, found := CredentialsFromCtx(request.Context())
	if !found {
		return "", fmt.Errorf("no credentials in context: %w", models.ForbiddenError)
	}

	var requestOrganizationId string
	if request != nil {
		requestOrganizationId = request.URL.Query().Get("organization-id")
		if requestOrganizationId != "" {
			if err := ValidateUuid(requestOrganizationId); err != nil {
				return "", err
			}
		}
	}

	// allow organizationId to be passed in query param
	if requestOrganizationId != "" {
		if err := EnforceOrganizationAccess(creds, requestOrganizationId); err != nil {
			return "", err
		}
		return requestOrganizationId, nil
	}

	if creds.OrganizationId == "" {
		noMarbleAdmin := ""
		if creds.Role == models.MARBLE_ADMIN {
			noMarbleAdmin = "this Api is not supposed to be called with marble admin creds "
		}
		return "", fmt.Errorf("no organizationId in context. %s: %w", noMarbleAdmin, models.ForbiddenError)
	}

	return creds.OrganizationId, nil
}

// TODO: replace me with OrganizationIdFromContext
func OrgIDFromCtx(ctx context.Context, request *http.Request) (organizationId string, err error) {
	return OrganizationIdFromRequest(request)
}
