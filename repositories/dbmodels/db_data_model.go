package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbDataModelTable struct {
	ID             string    `db:"id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	Name           string    `db:"name"`
	Description    string    `db:"description"`
	FTMEntity      *string   `db:"ftm_entity"`
	Alias          string    `db:"alias"`
	SemanticType   string    `db:"semantic_type"`
	CaptionField   string    `db:"caption_field"`
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
		fmtEntity = &entity
	}

	return models.TableMetadata{
		ID:             dbDataModelTable.ID,
		OrganizationID: dbDataModelTable.OrganizationID,
		Name:           dbDataModelTable.Name,
		Description:    dbDataModelTable.Description,
		FTMEntity:      fmtEntity,
		Alias:          dbDataModelTable.Alias,
		SemanticType:   models.SemanticType(dbDataModelTable.SemanticType),
		CaptionField:   dbDataModelTable.CaptionField,
	}, nil
}

type DbDataModelTableJoinField struct {
	TableID           string    `db:"data_model_tables.id"`
	OrganizationID    uuid.UUID `db:"data_model_tables.organization_id"`
	TableName         string    `db:"data_model_tables.name"`
	TableDescription  string    `db:"data_model_tables.description"`
	TableFTMEntity    *string   `db:"data_model_tables.ftm_entity"`
	TableAlias        string    `db:"data_model_tables.alias"`
	TableSemanticType string    `db:"data_model_tables.semantic_type"`
	TableCaptionField string    `db:"data_model_tables.caption_field"`
	FieldID           string    `db:"data_model_fields.id"`
	FieldName         string    `db:"data_model_fields.name"`
	FieldType         string    `db:"data_model_fields.type"`
	FieldNullable     bool      `db:"data_model_fields.nullable"`
	FieldDescription  string    `db:"data_model_fields.description"`
	FieldIsEnum       bool      `db:"data_model_fields.is_enum"`
	FieldFTMProperty  *string   `db:"data_model_fields.ftm_property"`
	FieldArchived     bool      `db:"data_model_fields.archived"`
}

var SelectDataModelTableJoinFieldColumns = utils.ColumnList[DbDataModelTableJoinField]()

type DbDataModelLink struct {
	Id              string
	OrganizationId  uuid.UUID
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
