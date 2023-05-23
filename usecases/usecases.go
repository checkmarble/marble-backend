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

func (usecases *Usecases) MarbleTokenUseCase() MarbleTokenUseCase {
	repositories := usecases.Repositories
	return MarbleTokenUseCase{
		firebaseTokenRepository: repositories.FirebaseTokenRepository,
		marbleJwtRepository:     repositories.MarbleJwtRepository,
		userRepository:          repositories.UserRepository,
		apiKeyRepository:        repositories.ApiKeyRepository,
		tokenLifetimeMinute:     usecases.Config.TokenLifetimeMinute,
	}
}
