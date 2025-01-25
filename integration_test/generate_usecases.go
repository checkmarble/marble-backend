package integration

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
)

func generateUsecaseWithCredForMarbleAdmin(testUsecases usecases.Usecaser) usecases.UsecasesWithCreds {
	creds := models.Credentials{Role: models.MARBLE_ADMIN}
	return usecases.UsecasesWithCreds{
		Usecaser:    testUsecases,
		Credentials: creds,
	}
}

func generateUsecaseWithCreds(
	testUsecases usecases.Usecaser,
	creds models.Credentials,
) usecases.UsecasesWithCreds {
	return usecases.UsecasesWithCreds{
		Usecaser:    testUsecases,
		Credentials: creds,
	}
}
