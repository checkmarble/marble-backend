package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_FEATURES = "features"

var SelectFeatureColumn = utils.ColumnList[models.Feature]()

type DBFeature struct {
	Id        string    `db:"id"`
	Name      string    `db:"name"`
	Slug      string    `db:"slug"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func AdaptFeature(db DBFeature) (models.Feature, error) {
	return models.Feature{
		Id:   db.Id,
		Name: db.Name,
		Slug: db.Slug,
	}, nil
}

type DBFeatureCreateInput struct {
	Id   string `db:"id"`
	Name string `db:"name"`
}

func AdaptCreateFeatureInput(db DBFeatureCreateInput) models.CreateFeatureInput {
	return models.CreateFeatureInput{
		Id:   db.Id,
		Name: db.Name,
	}
}

type DBFeatureUpdateInput struct {
	Id   string `db:"id"`
	Name string `db:"name"`
}

func AdaptUpdateFeatureInput(db DBFeatureUpdateInput) models.UpdateFeatureInput {
	return models.UpdateFeatureInput{
		Id:   db.Id,
		Name: db.Name,
	}
}
