package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type CreateContinuousScreeningObjectDto struct {
	ObjectType     string           `json:"object_type" binding:"required"`
	ConfigStableId uuid.UUID        `json:"config_stable_id" binding:"required"`
	ObjectId       *string          `json:"object_id"`
	ObjectPayload  *json.RawMessage `json:"object_payload"`
}

func (dto CreateContinuousScreeningObjectDto) Validate() error {
	if dto.ObjectId == nil && dto.ObjectPayload == nil {
		return errors.Wrap(
			models.BadParameterError,
			"object_id or object_payload is required",
		)
	}

	if dto.ObjectId != nil && dto.ObjectPayload != nil {
		return errors.Wrap(
			models.BadParameterError,
			"object_id and object_payload cannot be provided together",
		)
	}

	return nil
}

func AdaptCreateContinuousScreeningObjectDto(dto CreateContinuousScreeningObjectDto) models.CreateContinuousScreeningObject {
	return models.CreateContinuousScreeningObject{
		ObjectType:     dto.ObjectType,
		ConfigStableId: dto.ConfigStableId,
		ObjectId:       dto.ObjectId,
		ObjectPayload:  dto.ObjectPayload,
	}
}

type DeleteContinuousScreeningObjectDto struct {
	ObjectType     string    `json:"object_type" binding:"required"`
	ObjectId       string    `json:"object_id" binding:"required"`
	ConfigStableId uuid.UUID `json:"config_stable_id" binding:"required"`
}

func AdaptDeleteContinuousScreeningObjectDto(dto DeleteContinuousScreeningObjectDto) models.DeleteContinuousScreeningObject {
	return models.DeleteContinuousScreeningObject{
		ObjectType:     dto.ObjectType,
		ObjectId:       dto.ObjectId,
		ConfigStableId: dto.ConfigStableId,
	}
}

type ListContinuousScreeningObjectsFilters struct {
	ObjectTypes     []string    `form:"object_type[]"`
	ObjectIds       []string    `form:"object_id[]"`
	ConfigStableIds []uuid.UUID `form:"config_stable_id[]"`
	StartDate       string      `form:"start_date"`
	EndDate         string      `form:"end_date"`
}

func (dto ListContinuousScreeningObjectsFilters) Parse() (models.ListMonitoredObjectsFilters, error) {
	out := models.ListMonitoredObjectsFilters{
		ObjectTypes:     dto.ObjectTypes,
		ObjectIds:       dto.ObjectIds,
		ConfigStableIds: dto.ConfigStableIds,
	}

	if dto.StartDate != "" {
		startDate, err := time.Parse(time.RFC3339, dto.StartDate)
		if err != nil {
			return out, errors.Wrap(models.BadParameterError,
				"invalid start_date format, expected RFC3339")
		}
		out.StartDate = startDate
	}

	if dto.EndDate != "" {
		endDate, err := time.Parse(time.RFC3339, dto.EndDate)
		if err != nil {
			return out, errors.Wrap(models.BadParameterError,
				"invalid end_date format, expected RFC3339")
		}
		out.EndDate = endDate
	}

	return out, nil
}

type ContinuousScreeningObjectDto struct {
	Id             uuid.UUID `json:"id"`
	ObjectType     string    `json:"object_type"`
	ObjectId       string    `json:"object_id"`
	ConfigStableId uuid.UUID `json:"config_stable_id"`
	CreatedAt      time.Time `json:"created_at"`
}

func AdaptContinuousScreeningObjectDto(obj models.ContinuousScreeningMonitoredObject) ContinuousScreeningObjectDto {
	return ContinuousScreeningObjectDto{
		Id:             obj.Id,
		ObjectType:     obj.ObjectType,
		ObjectId:       obj.ObjectId,
		ConfigStableId: obj.ConfigStableId,
		CreatedAt:      obj.CreatedAt,
	}
}
