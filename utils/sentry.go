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
		hub.CaptureException(err)
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
