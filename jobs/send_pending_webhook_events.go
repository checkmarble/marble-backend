package jobs

import (
	"context"

	"github.com/checkmarble/marble-backend/usecases"
)

// Runs every minute
func SendPendingWebhookEvents(ctx context.Context, uc usecases.Usecases) error {
	return executeWithMonitoring(
		ctx,
		uc,
		"send-webhook-events",
		func(
			ctx context.Context, usecases usecases.Usecases,
		) error {
			usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
			webhooksUsecase := usecasesWithCreds.NewWebhookEventsUsecase()
			return webhooksUsecase.RetrySendWebhookEvents(ctx)
		},
	)
}
