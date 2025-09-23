package repositories

type MarbleDbRepository struct {
	withCache        bool
	trigramThreshold float64
}

func NewMarbleDbRepository(withCache bool, trigramThreshold float64) *MarbleDbRepository {
	return &MarbleDbRepository{
		withCache:        withCache,
		trigramThreshold: trigramThreshold,
	}
}
