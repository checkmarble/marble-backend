package utils

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/getsentry/sentry-go"
)

func LogAndReportSentryError(ctx context.Context, err error) {
	logger := LoggerFromContext(ctx)
	logger.ErrorContext(ctx, fmt.Sprintf("%+v", err))

	// Ignore errors that are due to context deadlines or canceled context, as presumably their root case has been handled
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		logger.DebugContext(ctx, fmt.Sprintf("Deadline exceeded or context canceled: %v", err))
		return
	}

	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		CaptureSentryException(ctx, hub, err)
	} else {
		sentry.CaptureException(err)
	}
}

func RecoverAndReportSentryError(ctx context.Context, callerName string) {
	if r := recover(); r != nil {
		logger := LoggerFromContext(ctx)
		logger.ErrorContext(ctx, fmt.Sprintf("Recovered from panic in %s", callerName))
		LogAndReportSentryError(ctx, errors.New(string(debug.Stack())))
	}
}

func CaptureSentryException(ctx context.Context, hub *sentry.Hub, err error) {
	creds, ok := CredentialsFromCtx(ctx)
	if ok {
		if creds.ActorIdentity.ApiKeyName != "" {
			hub.Scope().SetUser(sentry.User{
				Name: creds.ActorIdentity.ApiKeyName,
			})
		}
		if creds.ActorIdentity.UserId != "" {
			hub.Scope().SetUser(sentry.User{
				ID:       string(creds.ActorIdentity.UserId),
				Username: fmt.Sprintf("%s %s", creds.ActorIdentity.FirstName, creds.ActorIdentity.LastName),
				Email:    creds.ActorIdentity.Email,
			})
		}
		hub.Scope().SetTag("organization_id", creds.OrganizationId)
		hub.Scope().SetTag("role", creds.Role.String())
	}
	hub.CaptureException(err)
}
