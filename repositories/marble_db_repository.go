package repositories

type MarbleDbRepository struct {
	withCache bool
}

func NewMarbleDbRepository(withCache bool) *MarbleDbRepository {
	return &MarbleDbRepository{
		withCache: withCache,
	}
}
