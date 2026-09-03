package jobs

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/google/uuid"
)

func GenerateUsecaseWithCredForSystem(jobUsecases usecases.Usecases) usecases.UsecasesWithCreds {
	creds := models.Credentials{
		Roles:          []models.Role{models.SYSTEM},
		OrganizationId: uuid.Nil,
	}
	return usecases.UsecasesWithCreds{
		Usecases:    jobUsecases,
		Credentials: creds,
	}
}
