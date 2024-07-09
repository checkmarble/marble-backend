package jobs

import (
	"context"

	"github.com/checkmarble/marble-backend/usecases"
)

// Runs every minute
func SendPendingWebhooks(ctx context.Context, uc usecases.Usecases) error {
	return executeWithMonitoring(
		ctx,
		uc,
		"send-webhooks",
		func(
			ctx context.Context, usecases usecases.Usecases,
		) error {
			usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
			webhooksUsecase := usecasesWithCreds.NewWebhooksUsecase()
			return webhooksUsecase.SendWebhooks(ctx)
		},
	)
}
