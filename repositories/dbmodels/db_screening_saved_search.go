package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbScreeningSavedSearch struct {
	Id        uuid.UUID       `db:"id"`
	OrgId     uuid.UUID       `db:"org_id"`
	Provider  string          `db:"provider"`
	Config    json.RawMessage `db:"config"`
	CreatedAt time.Time       `db:"created_at"`
	DeletedAt *time.Time      `db:"deleted_at"`
}

const TABLE_SCREENING_SAVED_SEARCHES = "screening_saved_searches"

var ScreeningSavedSearchesColumns = utils.ColumnList[DbScreeningSavedSearch]()

func AdaptScreeningSavedSearch(db DbScreeningSavedSearch) (models.ScreeningSavedSearch, error) {
	return models.ScreeningSavedSearch{
		Id:        db.Id,
		OrgId:     db.OrgId,
		Provider:  db.Provider,
		Config:    db.Config,
		CreatedAt: db.CreatedAt,
		DeletedAt: db.DeletedAt,
	}, nil
}
