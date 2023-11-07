package utils

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/segmentio/analytics-go/v3"

	"github.com/checkmarble/marble-backend/models"
)

type ContextKey int

const (
	ContextKeyCredentials ContextKey = iota
	ContextKeyLogger
	ContextKeySegmentClient
)

func CredentialsFromCtx(ctx context.Context) models.Credentials {
	creds, _ := ctx.Value(ContextKeyCredentials).(models.Credentials)
	return creds
}

func OrganizationIdFromRequest(request *http.Request) (organizationId string, err error) {
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

func ValidateUuid(uuidParam string) error {
	_, err := uuid.FromString(uuidParam)
	if err != nil {
		err = fmt.Errorf("'%s' is not a valid UUID: %w", uuidParam, models.BadParameterError)
	}
	return err
}

func SegmentClientFromContext(ctx context.Context) analytics.Client {
	client, found := ctx.Value(ContextKeySegmentClient).(analytics.Client)
	if !found {
		logger := LoggerFromContext(ctx)
		logger.ErrorContext(ctx, "Segment client not found in context: creating a new one to avoid nil pointer panic but it will not work")
		client = analytics.New("")
	}
	return client
}

func StoreSegmentClientInContext(ctx context.Context, client analytics.Client) context.Context {
	return context.WithValue(ctx, ContextKeySegmentClient, client)
}

func StoreSegmentClientInContextMiddleware(client analytics.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctxWithSegment := StoreSegmentClientInContext(c.Request.Context(), client)
		c.Request = c.Request.WithContext(ctxWithSegment)
	}
}
