package jobs

import (
	"context"

	"github.com/checkmarble/marble-backend/usecases"
)

func SendPendingWebhookEvents(ctx context.Context, uc usecases.Usecases) {
	executeWithMonitoring(
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
