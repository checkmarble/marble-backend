package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

// ScreeningFreeformSearchResult is the response of performing or saving a freeform search. It
// carries the search id (so the frontend can later save the results) alongside the matches.
type ScreeningFreeformSearchResult struct {
	Id      uuid.UUID         `json:"id"`
	Matches []json.RawMessage `json:"matches"`
}

func AdaptScreeningFreeformSearchResult(id uuid.UUID, matches []models.ScreeningMatch) ScreeningFreeformSearchResult {
	return ScreeningFreeformSearchResult{
		Id:      id,
		Matches: pure_utils.Map(matches, func(m models.ScreeningMatch) json.RawMessage { return m.Payload }),
	}
}

// SavedScreeningFreeformSearch exposes a stored freeform search together with its saved results.
type SavedScreeningFreeformSearch struct {
	ScreeningFreeformSearch
	Matches []json.RawMessage `json:"matches"`
}

func AdaptSavedScreeningFreeformSearch(m models.FreeformSearch) SavedScreeningFreeformSearch {
	return SavedScreeningFreeformSearch{
		ScreeningFreeformSearch: AdaptScreeningFreeformSearchDto(m),
		Matches:                 m.Result,
	}
}

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
	NbHits       int                         `json:"nb_hits"`
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
		NbHits:       m.NbHits,
	}
}

type ScreeningFreeformSearchFilters struct {
	UserId        *string    `form:"user_id" binding:"omitempty,uuid"`
	ApiKeyId      *string    `form:"api_key_id" binding:"omitempty,uuid"`
	CreatedBefore *time.Time `form:"created_before"`
	CreatedAfter  *time.Time `form:"created_after"`

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
		OrgId:         orgId,
		UserId:        userId,
		ApiKeyId:      apiKeyId,
		CreatedBefore: f.CreatedBefore,
		CreatedAfter:  f.CreatedAfter,
		SavedOnly:     f.SavedOnly,
	}
}
