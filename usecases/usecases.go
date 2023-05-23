package usecases

import (
	"marble/marble-backend/repositories"
)

type Usecases struct {
	repositories repositories.Repositories
}

func NewUsecases(repositories repositories.Repositories) Usecases {
	return Usecases{
		repositories: repositories,
	}
}

func (usecases *Usecases) MarbleTokenUseCase() MarbleTokenUseCase {
	return MarbleTokenUseCase{
		firebaseTokenRepository: usecases.repositories.FirebaseTokenRepository,
		marbleJwtRepository:     usecases.repositories.MarbleJwtRepository,
		userRepository:          usecases.repositories.UserRepository,
		apiKeyRepository:        usecases.repositories.ApiKeyRepository,
	}
}
