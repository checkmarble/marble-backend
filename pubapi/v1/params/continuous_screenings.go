package params

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type CreateContinuousScreeningObjectParams struct {
	ObjectType     string    `json:"object_type" binding:"required"`
	ConfigStableId uuid.UUID `json:"config_stable_id" binding:"required"`
	ObjectId       string    `json:"object_id" binding:"required"`
	SkipScreen     bool      `json:"skip_screen"`
}

func (dto CreateContinuousScreeningObjectParams) ToModel() models.CreateContinuousScreeningObject {
	return models.CreateContinuousScreeningObject{
		ObjectType:     dto.ObjectType,
		ConfigStableId: dto.ConfigStableId,
		ObjectId:       dto.ObjectId,
		SkipScreen:     dto.SkipScreen,
	}
}

type DeleteContinuousScreeningObjectParams struct {
	ObjectType     string    `json:"object_type" binding:"required"`
	ObjectId       string    `json:"object_id" binding:"required"`
	ConfigStableId uuid.UUID `json:"config_stable_id" binding:"required"`
}

func (dto DeleteContinuousScreeningObjectParams) ToModel() models.DeleteContinuousScreeningObject {
	return models.DeleteContinuousScreeningObject{
		ObjectType:     dto.ObjectType,
		ObjectId:       dto.ObjectId,
		ConfigStableId: dto.ConfigStableId,
	}
}
