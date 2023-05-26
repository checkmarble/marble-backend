package usecases

import (
	"marble/marble-backend/repositories"
)

type Configuration struct {
	TokenLifetimeMinute int
}

type Usecases struct {
	Repositories repositories.Repositories
	Config       Configuration
}

func (usecases *Usecases) NewMarbleTokenUseCase() MarbleTokenUseCase {
	repositories := usecases.Repositories
	return MarbleTokenUseCase{
		firebaseTokenRepository: repositories.FirebaseTokenRepository,
		marbleJwtRepository:     repositories.MarbleJwtRepository,
		userRepository:          repositories.UserRepository,
		apiKeyRepository:        repositories.ApiKeyRepository,
		organizationRepository:  repositories.OrganizationRepository,
		tokenLifetimeMinute:     usecases.Config.TokenLifetimeMinute,
	}
}

func (usecases *Usecases) NewOrganizationUseCase() OrganizationUseCase {
	return OrganizationUseCase{
		organizationRepository: usecases.Repositories.OrganizationRepository,
		datamodelRepository:    usecases.Repositories.DataModelRepository,
	}
}

func (usecases *Usecases) NewIngestionUseCase() IngestionUseCase {
	return IngestionUseCase{
		ingestionRepository: usecases.Repositories.IngestionRepository,
	}
}
