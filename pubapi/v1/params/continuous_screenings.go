package params

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type CreateContinuousScreeningObjectParams struct {
	ObjectType     string           `json:"object_type" binding:"required"`
	ConfigStableId uuid.UUID        `json:"config_stable_id" binding:"required"`
	ObjectId       *string          `json:"object_id"`
	ObjectPayload  *json.RawMessage `json:"object_payload"`
}

func (dto CreateContinuousScreeningObjectParams) Validate() error {
	if dto.ObjectId == nil && dto.ObjectPayload == nil {
		return errors.WithDetail(
			pubapi.ErrInvalidPayload,
			"object_id or object_payload is required",
		)
	}

	if dto.ObjectId != nil && dto.ObjectPayload != nil {
		return errors.WithDetail(
			pubapi.ErrInvalidPayload,
			"object_id and object_payload cannot be provided together",
		)
	}

	return nil
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
