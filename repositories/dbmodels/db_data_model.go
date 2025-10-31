package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type DbDataModelTable struct {
	ID             string  `db:"id"`
	OrganizationID string  `db:"organization_id"`
	Name           string  `db:"name"`
	Description    string  `db:"description"`
	FTMEntity      *string `db:"ftm_entity"`
}

const (
	TableDataModelTables = "data_model_tables"
	TableDataModelFields = "data_model_fields"
)

var SelectDataModelTableColumns = utils.ColumnList[DbDataModelTable]()

func AdaptTableMetadata(dbDataModelTable DbDataModelTable) (models.TableMetadata, error) {
	var fmtEntity *models.FollowTheMoneyEntity
	if dbDataModelTable.FTMEntity != nil {
		entity := models.FollowTheMoneyEntityFrom(*dbDataModelTable.FTMEntity)
		if entity == models.FollowTheMoneyEntityUnknown {
			return models.TableMetadata{}, errors.Newf("invalid FTM entity: %s", *dbDataModelTable.FTMEntity)
		}
		fmtEntity = &entity
	}

	return models.TableMetadata{
		ID:             dbDataModelTable.ID,
		OrganizationID: dbDataModelTable.OrganizationID,
		Name:           dbDataModelTable.Name,
		Description:    dbDataModelTable.Description,
		FTMEntity:      fmtEntity,
	}, nil
}

type DbDataModelTableJoinField struct {
	TableID          string  `db:"data_model_tables.id"`
	OrganizationID   string  `db:"data_model_tables.organization_id"`
	TableName        string  `db:"data_model_tables.name"`
	TableDescription string  `db:"data_model_tables.description"`
	TableFTMEntity   *string `db:"data_model_tables.ftm_entity"`
	FieldID          string  `db:"data_model_fields.id"`
	FieldName        string  `db:"data_model_fields.name"`
	FieldType        string  `db:"data_model_fields.type"`
	FieldNullable    bool    `db:"data_model_fields.nullable"`
	FieldDescription string  `db:"data_model_fields.description"`
	FieldIsEnum      bool    `db:"data_model_fields.is_enum"`
	FieldFTMProperty *string `db:"data_model_fields.ftm_property"`
}

var SelectDataModelTableJoinFieldColumns = utils.ColumnList[DbDataModelTableJoinField]()

type DbDataModelLink struct {
	Id              string
	OrganizationId  string
	Name            string
	ParentTableName string
	ParentTableId   string
	ParentFieldName string
	ParentFieldId   string
	ChildTableName  string
	ChildTableId    string
	ChildFieldName  string
	ChildFieldId    string
}

func AdaptLinkToSingle(dbDataModelLink DbDataModelLink) models.LinkToSingle {
	return models.LinkToSingle{
		Id:              dbDataModelLink.Id,
		OrganizationId:  dbDataModelLink.OrganizationId,
		Name:            dbDataModelLink.Name,
		ParentTableName: dbDataModelLink.ParentTableName,
		ParentTableId:   dbDataModelLink.ParentTableId,
		ParentFieldName: dbDataModelLink.ParentFieldName,
		ParentFieldId:   dbDataModelLink.ParentFieldId,
		ChildTableName:  dbDataModelLink.ChildTableName,
		ChildTableId:    dbDataModelLink.ChildTableId,
		ChildFieldName:  dbDataModelLink.ChildFieldName,
		ChildFieldId:    dbDataModelLink.ChildFieldId,
	}
}
