package jobs

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

const JOB_ORG_ID string = "job"

func GenerateUsecaseWithCredForMarbleAdmin(ctx context.Context, jobUsecases usecases.Usecases) usecases.UsecasesWithCreds {
	creds := models.Credentials{
		Role:           models.MARBLE_ADMIN,
		OrganizationId: JOB_ORG_ID,
	}
	return usecases.UsecasesWithCreds{
		Usecases:                jobUsecases,
		Credentials:             creds,
		Logger:                  utils.LoggerFromContext(ctx),
		OrganizationIdOfContext: func() (string, error) { return JOB_ORG_ID, nil },
		Context:                 ctx,
	}
}
