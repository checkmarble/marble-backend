package dto

import (
	"encoding/json"
	"regexp"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

var regexpStableId = regexp.MustCompile(`^[a-zA-Z0-9_]{1,32}$`)

type ContinuousScreeningConfigDto struct {
	Id             uuid.UUID `json:"id"`
	StableId       string    `json:"stable_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	ObjectTypes    []string  `json:"object_types"`
	Algorithm      string    `json:"algorithm"`
	Datasets       []string  `json:"datasets"`
	MatchThreshold int       `json:"match_threshold"`
	MatchLimit     int       `json:"match_limit"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func AdaptContinuousScreeningConfigDto(config models.ContinuousScreeningConfig) ContinuousScreeningConfigDto {
	return ContinuousScreeningConfigDto{
		Id:             config.Id,
		StableId:       config.StableId,
		Name:           config.Name,
		Description:    config.Description,
		ObjectTypes:    config.ObjectTypes,
		Algorithm:      config.Algorithm,
		Datasets:       config.Datasets,
		MatchThreshold: config.MatchThreshold,
		MatchLimit:     config.MatchLimit,
		Enabled:        config.Enabled,
		CreatedAt:      config.CreatedAt,
		UpdatedAt:      config.UpdatedAt,
	}
}

type CreateContinuousScreeningConfigDto struct {
	Name           string   `json:"name" binding:"required"`
	Description    string   `json:"description"`
	StableId       string   `json:"stable_id" binding:"required"`
	Algorithm      string   `json:"algorithm" binding:"required"`
	Datasets       []string `json:"datasets" binding:"required"`
	MatchThreshold int      `json:"match_threshold" binding:"required"`
	MatchLimit     int      `json:"match_limit" binding:"required"`
	ObjectTypes    []string `json:"object_types" binding:"required"`
}

func (dto CreateContinuousScreeningConfigDto) Validate() error {
	if len(dto.Datasets) == 0 {
		return errors.Wrap(
			models.BadParameterError,
			"datasets are required for continuous screening config",
		)
	}

	if dto.MatchThreshold < 0 || dto.MatchThreshold > 100 {
		return errors.Wrap(
			models.BadParameterError,
			"match threshold must be between 0 and 100",
		)
	}

	if dto.MatchLimit < 1 {
		return errors.Wrap(
			models.BadParameterError,
			"match limit must be at least 1",
		)
	}

	if len(dto.ObjectTypes) == 0 {
		return errors.Wrap(
			models.BadParameterError,
			"object types are required for continuous screening config",
		)
	}

	// Check stableID is valid
	if !regexpStableId.MatchString(dto.StableId) {
		return errors.Wrap(
			models.BadParameterError,
			"stable ID must contain only letters, numbers, underscores and be at most 64 characters",
		)
	}

	return nil
}

func AdaptCreateContinuousScreeningConfigDtoToModel(dto CreateContinuousScreeningConfigDto) models.CreateContinuousScreeningConfig {
	return models.CreateContinuousScreeningConfig{
		Name:           dto.Name,
		StableId:       dto.StableId,
		ObjectTypes:    dto.ObjectTypes,
		Description:    dto.Description,
		Algorithm:      dto.Algorithm,
		Datasets:       dto.Datasets,
		MatchThreshold: dto.MatchThreshold,
		MatchLimit:     dto.MatchLimit,
	}
}

type UpdateContinuousScreeningConfigDto struct {
	Name           *string   `json:"name"`
	Description    *string   `json:"description"`
	Algorithm      *string   `json:"algorithm"`
	Datasets       *[]string `json:"datasets"`
	MatchThreshold *int      `json:"match_threshold"`
	MatchLimit     *int      `json:"match_limit"`
	Enabled        *bool     `json:"enabled"`
}

func (dto UpdateContinuousScreeningConfigDto) Validate() error {
	if dto.MatchThreshold != nil && (*dto.MatchThreshold < 0 || *dto.MatchThreshold > 100) {
		return errors.Wrap(
			models.BadParameterError,
			"match threshold must be between 0 and 100",
		)
	}

	if dto.MatchLimit != nil && *dto.MatchLimit < 1 {
		return errors.Wrap(
			models.BadParameterError,
			"match limit must be at least 1",
		)
	}

	if dto.Datasets != nil && len(*dto.Datasets) == 0 {
		return errors.Wrap(
			models.BadParameterError,
			"datasets cannot be empty",
		)
	}

	return nil
}

func AdaptUpdateContinuousScreeningConfigDtoToModel(dto UpdateContinuousScreeningConfigDto) models.UpdateContinuousScreeningConfig {
	return models.UpdateContinuousScreeningConfig{
		Name:           dto.Name,
		Description:    dto.Description,
		Algorithm:      dto.Algorithm,
		Datasets:       dto.Datasets,
		MatchThreshold: dto.MatchThreshold,
		MatchLimit:     dto.MatchLimit,
		Enabled:        dto.Enabled,
	}
}

type InsertContinuousScreeningObjectDto struct {
	ObjectType     string           `json:"object_type" binding:"required"`
	ConfigStableId string           `json:"config_stable_id" binding:"required"`
	ObjectId       *string          `json:"object_id"`
	ObjectPayload  *json.RawMessage `json:"object_payload"`
}

func (dto InsertContinuousScreeningObjectDto) Validate() error {
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

func AdaptInsertContinuousScreeningObjectDto(dto InsertContinuousScreeningObjectDto) models.InsertContinuousScreeningObject {
	return models.InsertContinuousScreeningObject{
		ObjectType:     dto.ObjectType,
		ConfigStableId: dto.ConfigStableId,
		ObjectId:       dto.ObjectId,
		ObjectPayload:  dto.ObjectPayload,
	}
}
