package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type ContinuousScreeningConfigDto struct {
	Id             uuid.UUID `json:"id"`
	StableId       uuid.UUID `json:"stable_id"`
	InboxId        uuid.UUID `json:"inbox_id"`
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
		InboxId:        config.InboxId,
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
	Name           string    `json:"name" binding:"required"`
	Description    string    `json:"description"`
	InboxId        uuid.UUID `json:"inbox_id" binding:"required"`
	Algorithm      string    `json:"algorithm" binding:"required"`
	Datasets       []string  `json:"datasets" binding:"required"`
	MatchThreshold int       `json:"match_threshold" binding:"required"`
	MatchLimit     int       `json:"match_limit" binding:"required"`
	ObjectTypes    []string  `json:"object_types" binding:"required"`
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

	return nil
}

func AdaptCreateContinuousScreeningConfigDtoToModel(dto CreateContinuousScreeningConfigDto) models.CreateContinuousScreeningConfig {
	return models.CreateContinuousScreeningConfig{
		Name:           dto.Name,
		InboxId:        dto.InboxId,
		ObjectTypes:    dto.ObjectTypes,
		Description:    dto.Description,
		Algorithm:      dto.Algorithm,
		Datasets:       dto.Datasets,
		MatchThreshold: dto.MatchThreshold,
		MatchLimit:     dto.MatchLimit,
	}
}

type UpdateContinuousScreeningConfigDto struct {
	Name           *string    `json:"name"`
	Description    *string    `json:"description"`
	InboxId        *uuid.UUID `json:"inbox_id"`
	Algorithm      *string    `json:"algorithm"`
	Datasets       *[]string  `json:"datasets"`
	MatchThreshold *int       `json:"match_threshold"`
	MatchLimit     *int       `json:"match_limit"`
	Enabled        *bool      `json:"enabled"`
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
		InboxId:        dto.InboxId,
		Algorithm:      dto.Algorithm,
		Datasets:       dto.Datasets,
		MatchThreshold: dto.MatchThreshold,
		MatchLimit:     dto.MatchLimit,
		Enabled:        dto.Enabled,
	}
}
