package repositories

import "github.com/checkmarble/marble-backend/models"

type MarbleDbRepository struct {
	withCache           bool
	similarityThreshold float64
}

func NewMarbleDbRepository(withCache bool, similarityThreshold float64) *MarbleDbRepository {
	if similarityThreshold == 0 {
		similarityThreshold = models.DEFAULT_SIMILARITY_THRESHOLD
	}

	return &MarbleDbRepository{
		withCache:           withCache,
		similarityThreshold: similarityThreshold,
	}
}
