package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

type SecretKeeper interface {
	Encrypt(context.Context, []byte) ([]byte, error)
	Decrypt(context.Context, []byte) ([]byte, error)
}

type MarbleDbRepository struct {
	withCache           bool
	similarityThreshold float64
	oidcSecretsKeeper   SecretKeeper
}

func NewMarbleDbRepository(withCache bool, similarityThreshold float64, oidcSecretsKeeper ...SecretKeeper) *MarbleDbRepository {
	if similarityThreshold == 0 {
		similarityThreshold = models.DEFAULT_SIMILARITY_THRESHOLD
	}

	repository := &MarbleDbRepository{
		withCache:           withCache,
		similarityThreshold: similarityThreshold,
	}
	if len(oidcSecretsKeeper) > 0 {
		repository.oidcSecretsKeeper = oidcSecretsKeeper[0]
	}
	return repository
}
