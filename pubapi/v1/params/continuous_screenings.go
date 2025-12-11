package params

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type CreateContinuousScreeningObjectParams struct {
	ObjectType     string           `json:"object_type" binding:"required"`
	ConfigStableId uuid.UUID        `json:"config_stable_id" binding:"required"`
	ObjectId       *string          `json:"object_id" binding:"required_without_all=ObjectPayload,excluded_with=ObjectPayload"`
	ObjectPayload  *json.RawMessage `json:"object_payload" binding:"required_without_all=ObjectId,excluded_with=ObjectId"`
}

func (dto CreateContinuousScreeningObjectParams) ToModel() models.CreateContinuousScreeningObject {
	return models.CreateContinuousScreeningObject{
		ObjectType:     dto.ObjectType,
		ConfigStableId: dto.ConfigStableId,
		ObjectId:       dto.ObjectId,
		ObjectPayload:  dto.ObjectPayload,
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
