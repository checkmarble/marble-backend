package repositories

import "github.com/checkmarble/marble-backend/models"

type MarbleDbRepository struct {
	redisClient         *RedisClient
	withCache           bool
	similarityThreshold float64
}

func NewMarbleDbRepository(withCache bool, similarityThreshold float64, redisClient *RedisClient) *MarbleDbRepository {
	if similarityThreshold == 0 {
		similarityThreshold = models.DEFAULT_SIMILARITY_THRESHOLD
	}

	return &MarbleDbRepository{
		redisClient:         redisClient,
		withCache:           withCache,
		similarityThreshold: similarityThreshold,
	}
}
