package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type FreeformSearch struct {
	Id           uuid.UUID
	OrgId        uuid.UUID
	UserId       *uuid.UUID
	ApiKeyId     *uuid.UUID
	Provider     ScreeningProvider
	CreatedAt    time.Time
	SearchInput  ScreeningRefineRequest
	SearchConfig FreeformSearchConfig
	Result       []json.RawMessage
	ResultHash   []byte
	IsSaved      bool
}

type FreeformSearchConfig struct {
	Provider ScreeningProvider      `json:"provider"`
	Filters  ScreeningConfigFilters `json:"filters"`

	Threshold *int `json:"threshold"`
	Limit     int  `json:"limit"`
}

type ScreeningFreeformSearchFilters struct {
	OrgId uuid.UUID

	UserId   *uuid.UUID
	ApiKeyId *uuid.UUID

	// include only freeform searches where the user saved the results. Default: return all
	SavedOnly bool
}
