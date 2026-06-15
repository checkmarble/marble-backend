package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const TABLE_FREEFORM_SEARCHES = "screening_freeform_searches"

var SelectFreeformSearchColumn = utils.ColumnList[DBFreeformSearch]()

type DBFreeformSearch struct {
	Id           uuid.UUID                `db:"id"`
	OrgId        uuid.UUID                `db:"org_id"`
	UserId       *uuid.UUID               `db:"user_id"`
	ApiKeyId     *uuid.UUID               `db:"api_key_id"`
	Provider     models.ScreeningProvider `db:"provider"`
	CreatedAt    time.Time                `db:"created_at"`
	SearchInput  json.RawMessage          `db:"search_input"`
	SearchConfig json.RawMessage          `db:"search_config"`
	Result       json.RawMessage          `db:"result"`
	ResultHash   []byte                   `db:"result_hash"`
	IsSaved      bool                     `db:"is_saved"`
	NbHits       int                      `db:"nb_hits"`
}

func AdaptFreeformSearch(db DBFreeformSearch) (models.FreeformSearch, error) {
	searchInputDb := DBScreeningRefineRequest{}
	if err := json.Unmarshal(db.SearchInput, &searchInputDb); err != nil {
		return models.FreeformSearch{}, err
	}
	searchInput := models.ScreeningRefineRequest{
		Type:          searchInputDb.Type,
		Query:         searchInputDb.Query,
		LimitOverride: searchInputDb.LimitOverride,
	}

	config := models.FreeformSearchConfig{}
	if err := json.Unmarshal(db.SearchConfig, &config); err != nil {
		return models.FreeformSearch{}, err
	}

	var result []json.RawMessage
	if len(db.Result) > 0 {
		if err := json.Unmarshal(db.Result, &result); err != nil {
			return models.FreeformSearch{}, errors.Wrap(err, "error while unmarshalling freeform search results into an array of json objects")
		}
	}

	return models.FreeformSearch{
		Id:           db.Id,
		OrgId:        db.OrgId,
		UserId:       db.UserId,
		ApiKeyId:     db.ApiKeyId,
		Provider:     db.Provider,
		CreatedAt:    db.CreatedAt,
		SearchInput:  searchInput,
		SearchConfig: config,
		Result:       result,
		ResultHash:   db.ResultHash,
		IsSaved:      db.IsSaved,
		NbHits:       db.NbHits,
	}, nil
}

type DBScreeningRefineRequest struct {
	Type          string                     `json:"type"`
	Query         models.OpenSanctionsFilter `json:"query"`
	LimitOverride *int                       `json:"limit_override,omitempty"`
}

func AdaptDBScreeningRefineRequest(r models.ScreeningRefineRequest) DBScreeningRefineRequest {
	return DBScreeningRefineRequest{
		Type:          r.Type,
		Query:         r.Query,
		LimitOverride: r.LimitOverride,
	}
}
