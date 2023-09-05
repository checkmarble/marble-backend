package integration

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

const TEST_ADMIN_ORG_ID string = "admin_test"

func GenerateUsecaseWithCredForMarbleAdmin(ctx context.Context, testUsecases usecases.Usecases) usecases.UsecasesWithCreds {
	creds := models.Credentials{
		Role:           models.MARBLE_ADMIN,
		OrganizationId: TEST_ADMIN_ORG_ID,
	}
	return usecases.UsecasesWithCreds{
		Usecases:                testUsecases,
		Credentials:             creds,
		Logger:                  utils.LoggerFromContext(ctx),
		OrganizationIdOfContext: func() (string, error) { return TEST_ADMIN_ORG_ID, nil },
		Context:                 ctx,
	}
}
