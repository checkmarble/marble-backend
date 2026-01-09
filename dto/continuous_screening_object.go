package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type CreateContinuousScreeningObjectDto struct {
	ObjectType     string           `json:"object_type" binding:"required"`
	ConfigStableId uuid.UUID        `json:"config_stable_id" binding:"required"`
	ObjectId       *string          `json:"object_id" binding:"required_without_all=ObjectPayload,excluded_with=ObjectPayload"`
	ObjectPayload  *json.RawMessage `json:"object_payload" binding:"required_without_all=ObjectId,excluded_with=ObjectId"`
	ShouldScreen   bool             `json:"should_screen"`
}

func AdaptCreateContinuousScreeningObjectDto(dto CreateContinuousScreeningObjectDto) models.CreateContinuousScreeningObject {
	return models.CreateContinuousScreeningObject{
		ObjectType:     dto.ObjectType,
		ConfigStableId: dto.ConfigStableId,
		ObjectId:       dto.ObjectId,
		ObjectPayload:  dto.ObjectPayload,
		ShouldScreen:   dto.ShouldScreen,
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
	StartDate       *time.Time  `form:"start_date"`
	EndDate         *time.Time  `form:"end_date"`
}

func AdaptListContinuousScreeningObjectsFiltersDto(dto ListContinuousScreeningObjectsFilters) models.ListMonitoredObjectsFilters {
	return models.ListMonitoredObjectsFilters{
		ObjectTypes:     dto.ObjectTypes,
		ObjectIds:       dto.ObjectIds,
		ConfigStableIds: dto.ConfigStableIds,
		StartDate:       dto.StartDate,
		EndDate:         dto.EndDate,
	}
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
