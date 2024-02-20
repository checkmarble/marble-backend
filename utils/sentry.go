package utils

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
)

func LogAndReportSentryError(ctx context.Context, err error) {
	logger := LoggerFromContext(ctx)
	logger.ErrorContext(ctx, fmt.Sprintf("%+v", err))
	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		hub.CaptureException(err)
	} else {
		sentry.CaptureException(err)
	}
}
