package jobs

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/google/uuid"
)

func GenerateUsecaseWithCredForMarbleAdmin(ctx context.Context, jobUsecases usecases.Usecases) usecases.UsecasesWithCreds {
	creds := models.Credentials{
		Role:           models.MARBLE_ADMIN,
		OrganizationId: uuid.Nil,
	}
	return usecases.UsecasesWithCreds{
		Usecases:    jobUsecases,
		Credentials: creds,
	}
}
