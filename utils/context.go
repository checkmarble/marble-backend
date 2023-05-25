package utils

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
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

func OrgIDFromCtx(ctx context.Context) (id string, err error) {
	creds := CredentialsFromCtx(ctx)
	if creds.OrganizationId == "" {
		noMarbleAdmin := ""
		if creds.Role == models.MARBLE_ADMIN {
			noMarbleAdmin = "this Api is not supposed to be called with marble admin creds "
		}
		return "", fmt.Errorf("no organizationId in context. %s: %w", noMarbleAdmin, models.ForbiddenError)
	}
	return creds.OrganizationId, nil
}
