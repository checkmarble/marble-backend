package dbmodels

import (
	"encoding/json"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgtype"
)

// TODO(data_model): handle versionning + status / change db schema if it's useless
type DbDataModel struct {
	Id             string      `db:"id"`
	OrganizationId string      `db:"org_id"`
	Version        string      `db:"version"`
	Tables         []byte      `db:"tables"`
	DeletedAt      pgtype.Time `db:"deleted_at"`
}

const TABLE_DATA_MODELS = "data_models"

var SelectDataModelColumn = utils.ColumnList[DbDataModel]()

func AdaptDataModel(dbDataModel DbDataModel) (models.DataModel, error) {
	var tables map[models.TableName]models.Table
	if err := json.Unmarshal(dbDataModel.Tables, &tables); err != nil {
		return models.DataModel{}, fmt.Errorf("unable to unmarshal data model tables: %w", err)
	}

	return models.DataModel{
		Version: dbDataModel.Version,
		Tables:  tables,
	}, nil
}

type DbDataModelTable struct {
	ID             string `db:"id"`
	OrganizationID string `db:"organization_id"`
	Name           string `db:"name"`
	Description    string `db:"description"`
}

const (
	TableDataModelTables = "data_model_tables"
	TableDataModelFields = "data_model_fields"
)

var SelectDataModelTableColumns = utils.ColumnList[DbDataModelTable]()

func AdaptTableMetadata(dbDataModelTable DbDataModelTable) (models.TableMetadata, error) {
	return models.TableMetadata{
		ID:             dbDataModelTable.ID,
		OrganizationID: dbDataModelTable.OrganizationID,
		Name:           dbDataModelTable.Name,
		Description:    dbDataModelTable.Description,
	}, nil
}

type DbDataModelTableJoinField struct {
	TableID          string `db:"data_model_tables.id"`
	OrganizationID   string `db:"data_model_tables.organization_id"`
	TableName        string `db:"data_model_tables.name"`
	TableDescription string `db:"data_model_tables.description"`
	FieldID          string `db:"data_model_fields.id"`
	FieldName        string `db:"data_model_fields.name"`
	FieldType        string `db:"data_model_fields.type"`
	FieldNullable    bool   `db:"data_model_fields.nullable"`
	FieldDescription string `db:"data_model_fields.description"`
	FieldIsEnum      bool   `db:"data_model_fields.is_enum"`
}

var SelectDataModelTableJoinFieldColumns = utils.ColumnList[DbDataModelTableJoinField]()

type DbDataModelLink struct {
	Name        string
	ParentTable string
	ParentField string
	ChildTable  string
	ChildField  string
}

func AdaptLinkToSingle(dbDataModelLink DbDataModelLink) models.LinkToSingle {
	return models.LinkToSingle{
		Name:            dbDataModelLink.Name,
		LinkedTableName: models.TableName(dbDataModelLink.ParentTable),
		ParentFieldName: models.FieldName(dbDataModelLink.ParentField),
		ChildTableName:  models.TableName(dbDataModelLink.ChildTable),
		ChildFieldName:  models.FieldName(dbDataModelLink.ChildField),
	}
}
