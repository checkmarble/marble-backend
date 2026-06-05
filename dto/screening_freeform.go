package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type ScreeningFreeformSearch struct {
	Id           uuid.UUID                   `json:"id"`
	UserId       *uuid.UUID                  `json:"user_id,omitempty"`
	ApiKeyId     *uuid.UUID                  `json:"api_key_id,omitempty"`
	CreatedAt    time.Time                   `json:"created_at"`
	SearchInput  FreeformSearchInput         `json:"search_input"`
	SearchConfig models.FreeformSearchConfig `json:"search_config"`
}

type FreeformSearchInput struct {
	Type  string                     `json:"type"`
	Query models.OpenSanctionsFilter `json:"query"`
}

func (f FreeformSearchInput) ToScreeningRefineRequest() models.ScreeningRefineRequest {
	return models.ScreeningRefineRequest{
		Type:  f.Type,
		Query: f.Query,
	}
}

func adaptFreeformSearchInputDto(m models.ScreeningRefineRequest) FreeformSearchInput {
	return FreeformSearchInput{
		Type:  m.Type,
		Query: m.Query,
	}
}

func AdaptScreeningFreeformSearchDto(m models.FreeformSearch) ScreeningFreeformSearch {
	return ScreeningFreeformSearch{
		Id:           m.Id,
		UserId:       m.UserId,
		ApiKeyId:     m.ApiKeyId,
		CreatedAt:    m.CreatedAt,
		SearchInput:  adaptFreeformSearchInputDto(m.SearchInput),
		SearchConfig: m.SearchConfig,
	}
}

type ScreeningFreeformSearchFilters struct{}

func (f ScreeningFreeformSearchFilters) ToModel(orgId uuid.UUID) models.ScreeningFreeformSearchFilters {
	return models.ScreeningFreeformSearchFilters{
		OrgId: orgId,
	}
}
