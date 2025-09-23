package repositories

type MarbleDbRepository struct {
	withCache           bool
	similarityThreshold float64
}

func NewMarbleDbRepository(withCache bool, similarityThreshold float64) *MarbleDbRepository {
	return &MarbleDbRepository{
		withCache:           withCache,
		similarityThreshold: similarityThreshold,
	}
}
