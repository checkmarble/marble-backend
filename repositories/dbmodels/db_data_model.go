package dbmodels

import (
	"encoding/json"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgtype"
)

// TODO(data_model): handle versionning + status / change db schema if it's useless
type DbDataModel struct {
	ID        string      `db:"id"`
	OrgID     string      `db:"org_id"`
	Version   string      `db:"version"`
	Status    string      `db:"status"`
	Tables    []byte      `db:"tables"`
	DeletedAt pgtype.Time `db:"deleted_at"`
}

const TABLE_DATA_MODELS = "data_models"

var SelectDataModelColumn = utils.ColumnList[DbDataModel]()

func AdaptDataModel(dbDataModel DbDataModel) models.DataModel {
	var tables map[models.TableName]models.Table
	if err := json.Unmarshal(dbDataModel.Tables, &tables); err != nil {
		// who want to recover from malformed data: let's panic
		panic(fmt.Errorf("unable to unmarshal data model tables: %w", err))
	}

	return models.DataModel{
		Version: dbDataModel.Version,
		Status:  models.StatusFrom(dbDataModel.Status),
		Tables:  tables,
	}
}
