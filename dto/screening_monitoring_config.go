package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type ScreeningMonitoringConfigDto struct {
	Id             string    `json:"id"`
	Name           string    `json:"name"`
	Description    *string   `json:"description"`
	Datasets       []string  `json:"datasets"`
	MatchThreshold int       `json:"match_threshold"`
	MatchLimit     int       `json:"match_limit"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func AdaptScreeningMonitoringConfigDto(config models.ScreeningMonitoringConfig) ScreeningMonitoringConfigDto {
	return ScreeningMonitoringConfigDto{
		Id:             config.Id.String(),
		Name:           config.Name,
		Description:    config.Description,
		Datasets:       config.Datasets,
		MatchThreshold: config.MatchThreshold,
		MatchLimit:     config.MatchLimit,
		Enabled:        config.Enabled,
		CreatedAt:      config.CreatedAt,
		UpdatedAt:      config.UpdatedAt,
	}
}

type CreateScreeningMonitoringConfigDto struct {
	Name           string   `json:"name" binding:"required"`
	Description    *string  `json:"description"`
	Datasets       []string `json:"datasets" binding:"required"`
	MatchThreshold int      `json:"match_threshold" binding:"required"`
	MatchLimit     int      `json:"match_limit" binding:"required"`
}

func (dto CreateScreeningMonitoringConfigDto) Validate() error {
	if len(dto.Datasets) == 0 {
		return errors.New("datasets are required for screening monitoring config")
	}

	if dto.MatchThreshold < 0 || dto.MatchThreshold > 100 {
		return errors.New("match threshold must be between 0 and 100")
	}

	if dto.MatchLimit < 1 {
		return errors.New("match limit must be greater than or equal to 0")
	}

	return nil
}

func AdaptCreateScreeningMonitoringConfigDtoToModel(dto CreateScreeningMonitoringConfigDto) models.CreateScreeningMonitoringConfig {
	return models.CreateScreeningMonitoringConfig{
		Name:           dto.Name,
		Description:    dto.Description,
		Datasets:       dto.Datasets,
		MatchThreshold: dto.MatchThreshold,
		MatchLimit:     dto.MatchLimit,
	}
}

type UpdateScreeningMonitoringConfigDto struct {
	Name           *string   `json:"name"`
	Description    *string   `json:"description"`
	Datasets       *[]string `json:"datasets"`
	MatchThreshold *int      `json:"match_threshold"`
	MatchLimit     *int      `json:"match_limit"`
	Enabled        *bool     `json:"enabled"`
}

func (dto UpdateScreeningMonitoringConfigDto) Validate() error {
	if dto.MatchThreshold != nil && (*dto.MatchThreshold < 0 || *dto.MatchThreshold > 100) {
		return errors.New("match threshold must be between 0 and 100")
	}

	if dto.MatchLimit != nil && *dto.MatchLimit < 0 {
		return errors.New("match limit must be greater than or equal to 0")
	}

	if dto.Datasets != nil && len(*dto.Datasets) == 0 {
		return errors.New("datasets cannot be empty")
	}

	return nil
}

func AdaptUpdateScreeningMonitoringConfigDtoToModel(dto UpdateScreeningMonitoringConfigDto) models.UpdateScreeningMonitoringConfig {
	return models.UpdateScreeningMonitoringConfig{
		Name:           dto.Name,
		Description:    dto.Description,
		Datasets:       dto.Datasets,
		MatchThreshold: dto.MatchThreshold,
		MatchLimit:     dto.MatchLimit,
		Enabled:        dto.Enabled,
	}
}

type InsertScreeningMonitoringObjectDto struct {
	ObjectType    string           `json:"object_type" binding:"required"`
	ConfigId      uuid.UUID        `json:"config_id" binding:"required"`
	ObjectId      *string          `json:"object_id"`
	ObjectPayload *json.RawMessage `json:"object_payload"`
}

func (dto InsertScreeningMonitoringObjectDto) Validate() error {
	if dto.ObjectId == nil && dto.ObjectPayload == nil {
		return errors.New("object_id or object_payload is required")
	}

	if dto.ObjectId != nil && dto.ObjectPayload != nil {
		return errors.New("object_id and object_payload cannot be provided together")
	}

	return nil
}

func AdaptInsertScreeningMonitoringObjectDtoToModel(dto InsertScreeningMonitoringObjectDto) models.InsertScreeningMonitoringObject {
	return models.InsertScreeningMonitoringObject{
		ObjectType:    dto.ObjectType,
		ConfigId:      dto.ConfigId,
		ObjectId:      dto.ObjectId,
		ObjectPayload: dto.ObjectPayload,
	}
}
