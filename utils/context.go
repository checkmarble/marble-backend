package utils

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"net/http"

	"github.com/gofrs/uuid"
	"golang.org/x/exp/slog"
)

type ContextKey int

const (
	ContextKeyCredentials ContextKey = iota
	ContextKeyLogger
)

func loggerFromContext(ctx context.Context) *slog.Logger {
	logger, found := ctx.Value(ContextKeyLogger).(*slog.Logger)
	if !found {
		panic(fmt.Errorf("logger not found context"))
	}
	return logger
}

func LogRequestError(r *http.Request, msg string, args ...any) {
	ctx := r.Context()
	loggerFromContext(ctx).ErrorCtx(ctx, msg, args...)
}

func CredentialsFromCtx(ctx context.Context) (models.Credentials, bool) {

	creds, found := ctx.Value(ContextKeyCredentials).(models.Credentials)
	return creds, found
}

func MustCredentialsFromCtx(ctx context.Context) models.Credentials {

	creds, found := CredentialsFromCtx(ctx)
	if !found {
		panic(fmt.Errorf("credentials not found in request context"))
	}
	return creds
}

func OrgIDFromCtx(ctx context.Context, request *http.Request) (organizationID string, err error) {

	creds := MustCredentialsFromCtx(ctx)

	var requestOrganizationId string
	if request != nil {
		requestOrganizationId = request.URL.Query().Get("organization-id")
		if err := ValidateUuid(requestOrganizationId); err != nil {
			return "", err
		}
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

func ValidateUuid(uuidParam string) error {
	_, err := uuid.FromString(uuidParam)
	if err != nil {
		err = fmt.Errorf("'%s' is not a valid UUID: %w", uuidParam, models.BadParameterError)
	}
	return err
}
