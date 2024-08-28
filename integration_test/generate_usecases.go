package integration

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
)

func generateUsecaseWithCredForMarbleAdmin(
	testUsecases usecases.Usecases,
	organizationId string,
) usecases.UsecasesWithCreds {
	creds := models.Credentials{Role: models.MARBLE_ADMIN}
	return usecases.UsecasesWithCreds{
		Usecases:    testUsecases,
		Credentials: creds,
	}
}

func generateUsecaseWithCreds(
	testUsecases usecases.Usecases,
	creds models.Credentials,
) usecases.UsecasesWithCreds {
	return usecases.UsecasesWithCreds{
		Usecases:    testUsecases,
		Credentials: creds,
	}
}
