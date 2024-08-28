package jobs

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
)

func GenerateUsecaseWithCredForMarbleAdmin(ctx context.Context, jobUsecases usecases.Usecases) usecases.UsecasesWithCreds {
	creds := models.Credentials{
		Role:           models.MARBLE_ADMIN,
		OrganizationId: "",
	}
	return usecases.UsecasesWithCreds{
		Usecases:    jobUsecases,
		Credentials: creds,
	}
}
