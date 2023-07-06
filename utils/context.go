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

func StoreLoggerInContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ContextKeyLogger, logger)
}

func StoreLoggerInContextMiddleware(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctxWithLogger := StoreLoggerInContext(r.Context(), logger)
			next.ServeHTTP(w, r.WithContext(ctxWithLogger))
		})
	}
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	logger, found := ctx.Value(ContextKeyLogger).(*slog.Logger)
	if !found {
		panic(fmt.Errorf("logger not found context"))
	}
	return logger
}

func LogRequestError(r *http.Request, msg string, args ...any) {
	ctx := r.Context()
	LoggerFromContext(ctx).ErrorCtx(ctx, msg, args...)
}

func CredentialsFromCtx(ctx context.Context) models.Credentials {

	creds, _ := ctx.Value(ContextKeyCredentials).(models.Credentials)
	return creds
}

func OrganizationIdFromRequest(request *http.Request) (organizationID string, err error) {

	creds := CredentialsFromCtx(request.Context())

	var requestOrganizationId string
	if request != nil {
		requestOrganizationId = request.URL.Query().Get("organization-id")
		if requestOrganizationId != "" {
			if err := ValidateUuid(requestOrganizationId); err != nil {
				return "", err
			}
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

// TODO: replace me with OrganizationIdFromContext
func OrgIDFromCtx(ctx context.Context, request *http.Request) (organizationID string, err error) {
	return OrganizationIdFromRequest(request)
}

func ValidateUuid(uuidParam string) error {
	_, err := uuid.FromString(uuidParam)
	if err != nil {
		err = fmt.Errorf("'%s' is not a valid UUID: %w", uuidParam, models.BadParameterError)
	}
	return err
}
