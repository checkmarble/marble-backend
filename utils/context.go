package utils

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"net/http"
)

type ContextKey int

const (
	ContextKeyCredentials ContextKey = iota
)

func CredentialsFromCtx(ctx context.Context) models.Credentials {

	creds, found := ctx.Value(ContextKeyCredentials).(models.Credentials)

	if !found {
		panic(fmt.Errorf("credentials not found in request context"))
	}

	return creds
}

func OrgIDFromCtx(ctx context.Context, request *http.Request) (organizationID string, err error) {

	creds := CredentialsFromCtx(ctx)

	var requestOrganizationId string
	if request != nil {
		requestOrganizationId = request.URL.Query().Get("organization-id")
	}

	// allow orgId to be passed in query param
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
