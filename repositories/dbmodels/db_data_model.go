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
	Status         string      `db:"status"`
	Tables         []byte      `db:"tables"`
	DeletedAt      pgtype.Time `db:"deleted_at"`
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

type DbDataModelTable struct {
	ID             string `db:"id"`
	OrganizationID string `db:"organization_id"`
	Name           string `db:"name"`
	Description    string `db:"description"`
}

const TableDataModelTable = "data_model_tables"
const TableDataModelFields = "data_model_fields"

var SelectDataModelTableColumns = utils.ColumnList[DbDataModelTable]()

func AdaptDataModelTable(dbDataModelTable DbDataModelTable) models.DataModelTable {
	return models.DataModelTable{
		ID:             dbDataModelTable.ID,
		OrganizationID: dbDataModelTable.OrganizationID,
		Name:           dbDataModelTable.Name,
		Description:    dbDataModelTable.Description,
	}
}

type DbDataModelField struct {
	TableID          string `db:"data_model_tables.id"`
	OrganizationID   string `db:"data_model_tables.organization_id"`
	TableName        string `db:"data_model_tables.name"`
	TableDescription string `db:"data_model_tables.description"`
	FieldID          string `db:"data_model_fields.id"`
	FieldName        string `db:"data_model_fields.name"`
	FieldType        string `db:"data_model_fields.type"`
	FieldNullable    bool   `db:"data_model_fields.nullable"`
	FieldDescription string `db:"data_model_fields.description"`
}

var SelectDataModelFieldColumns = utils.ColumnList[DbDataModelField]()

func AdaptDataModelTableField(dbDataModelTableField DbDataModelField) models.DataModelTableField {
	return models.DataModelTableField{
		TableID:          dbDataModelTableField.TableID,
		OrganizationID:   dbDataModelTableField.OrganizationID,
		TableName:        dbDataModelTableField.TableName,
		TableDescription: dbDataModelTableField.TableDescription,
		FieldID:          dbDataModelTableField.FieldID,
		FieldName:        dbDataModelTableField.FieldName,
		FieldType:        dbDataModelTableField.FieldType,
		FieldNullable:    dbDataModelTableField.FieldNullable,
		FieldDescription: dbDataModelTableField.FieldDescription,
	}
}

type DataModelLink struct {
	ID            string
	Name          string
	ParentTableID string
	ParentTable   string
	ParentFieldID string
	ParentField   string
	ChildTableID  string
	ChildTable    string
	ChildFieldID  string
	ChildField    string
}

func AdaptDataModelLink(dbDataModelLink DataModelLink) models.DataModelLink {
	return models.DataModelLink{
		ID:          dbDataModelLink.ID,
		Name:        models.LinkName(dbDataModelLink.Name),
		ParentTable: models.TableName(dbDataModelLink.ParentTable),
		ParentField: models.FieldName(dbDataModelLink.ParentField),
		ChildTable:  models.TableName(dbDataModelLink.ChildTable),
		ChildField:  models.FieldName(dbDataModelLink.ChildField),
	}
}
