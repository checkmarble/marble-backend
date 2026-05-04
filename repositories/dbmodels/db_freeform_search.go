package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_FREEFORM_SEARCHES = "screening_freeform_searches"

var SelectFreeformSearchColumn = utils.ColumnList[DBFreeformSearch]()

type DBFreeformSearch struct {
	Id          uuid.UUID       `db:"id"`
	OrgId       uuid.UUID       `db:"org_id"`
	UserId      *uuid.UUID      `db:"user_id"`
	ApiKeyId    *uuid.UUID      `db:"api_key_id"`
	Provider    string          `db:"provider"`
	CreatedAt   time.Time       `db:"created_at"`
	SearchInput json.RawMessage `db:"search_input"`
	Result      json.RawMessage `db:"result"`
}

func AdaptFreeformSearch(db DBFreeformSearch) (models.FreeformSearch, error) {
	return models.FreeformSearch{
		Id:          db.Id,
		OrgId:       db.OrgId,
		UserId:      db.UserId,
		ApiKeyId:    db.ApiKeyId,
		Provider:    db.Provider,
		CreatedAt:   db.CreatedAt,
		SearchInput: db.SearchInput,
		Result:      db.Result,
	}, nil
}
