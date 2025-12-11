package dto

import (
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const DefaultContinuousScreeningAlgorithm = "best"

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

type ContinuousScreeningMappingFieldDto struct {
	ObjectFieldId uuid.UUID `json:"object_field_id" binding:"required"`
	FtmProperty   string    `json:"ftm_property" binding:"required"`
}

func AdaptContinuousScreeningMappingFieldDtoToModel(dto ContinuousScreeningMappingFieldDto) models.ContinuousScreeningMappingField {
	return models.ContinuousScreeningMappingField{
		ObjectFieldId: dto.ObjectFieldId,
		FtmProperty:   models.FollowTheMoneyPropertyFrom(dto.FtmProperty),
	}
}

type ContinuousScreeningMappingConfigDto struct {
	ObjectType          string                               `json:"object_type" binding:"required"`
	FtmEntity           string                               `json:"ftm_entity" binding:"required"`
	ObjectFieldMappings []ContinuousScreeningMappingFieldDto `json:"object_field_mappings"`
}

func (mapping ContinuousScreeningMappingConfigDto) Validate() error {
	// Check each mapping config
	// 1. Check if the entity is valid
	// 2. For each field mapping, check if the property is valid and belongs to the entity
	entity := models.FollowTheMoneyEntityFrom(mapping.FtmEntity)
	if entity == models.FollowTheMoneyEntityUnknown {
		return errors.Wrap(
			models.BadParameterError,
			"invalid FTM entity in mapping config",
		)
	}
	for _, fieldMapping := range mapping.ObjectFieldMappings {
		property := models.FollowTheMoneyPropertyFrom(fieldMapping.FtmProperty)
		if property == models.FollowTheMoneyPropertyUnknown {
			return errors.Wrap(
				models.BadParameterError,
				"invalid FTM property in mapping field",
			)
		}
		if !slices.Contains(models.FollowTheMoneyEntityProperties[entity], property) {
			return errors.Wrap(
				models.BadParameterError,
				"FTM property does not belong to the specified FTM entity",
			)
		}
	}
	return nil
}

func AdaptContinuousScreeningMappingConfigDtoToModel(dto ContinuousScreeningMappingConfigDto) models.ContinuousScreeningMappingConfig {
	return models.ContinuousScreeningMappingConfig{
		ObjectType: dto.ObjectType,
		FtmEntity:  models.FollowTheMoneyEntityFrom(dto.FtmEntity),
		ObjectFieldMappings: pure_utils.Map(
			dto.ObjectFieldMappings,
			AdaptContinuousScreeningMappingFieldDtoToModel,
		),
	}
}

type CreateContinuousScreeningConfigDto struct {
	Name           string                                `json:"name" binding:"required"`
	Description    string                                `json:"description"`
	InboxId        *uuid.UUID                            `json:"inbox_id"`
	InboxName      *string                               `json:"inbox_name"`
	Algorithm      *string                               `json:"algorithm"`
	Datasets       []string                              `json:"datasets" binding:"required"`
	MatchThreshold int                                   `json:"match_threshold" binding:"required"`
	MatchLimit     int                                   `json:"match_limit" binding:"required"`
	ObjectTypes    []string                              `json:"object_types" binding:"required"`
	MappingConfigs []ContinuousScreeningMappingConfigDto `json:"mapping_configs"`
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

	if dto.InboxId == nil && (dto.InboxName == nil || *dto.InboxName == "") {
		return errors.Wrap(
			models.BadParameterError,
			"either inbox_id or inbox_name must be provided",
		)
	}

	if dto.InboxId != nil && dto.InboxName != nil {
		return errors.Wrap(
			models.BadParameterError,
			"only one of inbox_id or inbox_name should be provided",
		)
	}

	if len(dto.ObjectTypes) == 0 {
		return errors.Wrap(
			models.BadParameterError,
			"object_types cannot be empty",
		)
	}

	// Check each mapping config
	// 1. Check if the entity is valid
	// 2. For each field mapping, check if the property is valid and belongs to the entity
	for _, mappingConfig := range dto.MappingConfigs {
		if err := mappingConfig.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func AdaptCreateContinuousScreeningConfigDtoToModel(dto CreateContinuousScreeningConfigDto) models.CreateContinuousScreeningConfig {
	if dto.Algorithm == nil {
		dto.Algorithm = utils.Ptr(DefaultContinuousScreeningAlgorithm)
	}
	return models.CreateContinuousScreeningConfig{
		Name:           dto.Name,
		InboxId:        dto.InboxId,
		InboxName:      dto.InboxName,
		Description:    dto.Description,
		Algorithm:      *dto.Algorithm,
		Datasets:       dto.Datasets,
		MatchThreshold: dto.MatchThreshold,
		MatchLimit:     dto.MatchLimit,
		ObjectTypes:    dto.ObjectTypes,
		MappingConfigs: pure_utils.Map(
			dto.MappingConfigs,
			AdaptContinuousScreeningMappingConfigDtoToModel,
		),
	}
}

type UpdateContinuousScreeningConfigDto struct {
	Name           *string                               `json:"name"`
	Description    *string                               `json:"description"`
	InboxId        *uuid.UUID                            `json:"inbox_id"`
	InboxName      *string                               `json:"inbox_name"`
	Algorithm      *string                               `json:"algorithm"`
	Datasets       *[]string                             `json:"datasets"`
	MatchThreshold *int                                  `json:"match_threshold"`
	MatchLimit     *int                                  `json:"match_limit"`
	Enabled        *bool                                 `json:"enabled"`
	ObjectTypes    *[]string                             `json:"object_types"`
	MappingConfigs []ContinuousScreeningMappingConfigDto `json:"mapping_configs"`
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

	if dto.InboxId != nil && dto.InboxName != nil {
		return errors.Wrap(
			models.BadParameterError,
			"only one of inbox_id or inbox_name should be provided",
		)
	}

	if dto.ObjectTypes != nil && len(*dto.ObjectTypes) == 0 {
		return errors.Wrap(
			models.BadParameterError,
			"object_types cannot be empty",
		)
	}

	for _, mapping := range dto.MappingConfigs {
		if err := mapping.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func AdaptUpdateContinuousScreeningConfigDtoToModel(dto UpdateContinuousScreeningConfigDto) models.UpdateContinuousScreeningConfig {
	mappingConfigs := pure_utils.Map(dto.MappingConfigs,
		AdaptContinuousScreeningMappingConfigDtoToModel)

	return models.UpdateContinuousScreeningConfig{
		Name:           dto.Name,
		Description:    dto.Description,
		InboxId:        dto.InboxId,
		InboxName:      dto.InboxName,
		Algorithm:      dto.Algorithm,
		Datasets:       dto.Datasets,
		MatchThreshold: dto.MatchThreshold,
		MatchLimit:     dto.MatchLimit,
		Enabled:        dto.Enabled,
		ObjectTypes:    dto.ObjectTypes,
		MappingConfigs: mappingConfigs,
	}
}
