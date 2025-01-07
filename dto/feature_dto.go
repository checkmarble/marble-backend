package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type APIFeature struct {
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func AdaptFeatureDto(f models.Feature) APIFeature {
	return APIFeature{
		Id:        f.Id,
		Name:      f.Name,
		CreatedAt: f.CreatedAt,
	}
}

type CreateFeatureBody struct {
	Name string `json:"name" binding:"required"`
}

type UpdateFeatureBody struct {
	Name string `json:"name"`
}
