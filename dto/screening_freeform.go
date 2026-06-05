package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type PaginatedScreeningFreeformSearches struct {
	Data        []ScreeningFreeformSearch `json:"data"`
	HasNextPage bool                      `json:"has_next_page"`
}

func AdaptPaginatedScreeningFreeformSearches(data []models.FreeformSearch, hasNextPage bool) PaginatedScreeningFreeformSearches {
	items := make([]ScreeningFreeformSearch, len(data))
	for i, freeformSearch := range data {
		items[i] = AdaptScreeningFreeformSearchDto(freeformSearch)
	}

	return PaginatedScreeningFreeformSearches{
		Data:        items,
		HasNextPage: hasNextPage,
	}
}

type ScreeningFreeformSearch struct {
	Id           uuid.UUID                   `json:"id"`
	UserId       *uuid.UUID                  `json:"user_id,omitempty"`
	ApiKeyId     *uuid.UUID                  `json:"api_key_id,omitempty"`
	CreatedAt    time.Time                   `json:"created_at"`
	SearchInput  FreeformSearchInput         `json:"search_input"`
	SearchConfig models.FreeformSearchConfig `json:"search_config"`
	IsSaved      bool                        `json:"is_saved"`
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
		IsSaved:      m.IsSaved,
	}
}

type ScreeningFreeformSearchFilters struct {
	UserId   *string `form:"user_id" binding:"omitempty,uuid"`
	ApiKeyId *string `form:"api_key_id" binding:"omitempty,uuid"`

	// include only freeform searches where the user saved the results. Default: return all
	SavedOnly bool `form:"saved_only"`
}

func (f ScreeningFreeformSearchFilters) ToModel(orgId uuid.UUID) models.ScreeningFreeformSearchFilters {
	var userId *uuid.UUID
	if f.UserId != nil {
		userId = utils.Ptr(uuid.MustParse(*f.UserId))
	}
	var apiKeyId *uuid.UUID
	if f.ApiKeyId != nil {
		apiKeyId = utils.Ptr(uuid.MustParse(*f.ApiKeyId))
	}

	return models.ScreeningFreeformSearchFilters{
		OrgId:     orgId,
		UserId:    userId,
		ApiKeyId:  apiKeyId,
		SavedOnly: f.SavedOnly,
	}
}
