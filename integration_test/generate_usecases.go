package integration

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func GenerateUsecaseWithCredForMarbleAdmin(
	ctx context.Context,
	testUsecases usecases.Usecases,
	organizationId string,
) usecases.UsecasesWithCreds {
	creds := models.Credentials{
		Role:           models.MARBLE_ADMIN,
		OrganizationId: organizationId,
	}
	return usecases.UsecasesWithCreds{
		Usecases:                testUsecases,
		Credentials:             creds,
		Logger:                  utils.LoggerFromContext(ctx),
		OrganizationIdOfContext: func() (string, error) { return organizationId, nil },
		Context:                 ctx,
	}
}
