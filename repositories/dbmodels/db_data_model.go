package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbDataModelTable struct {
	ID             string  `db:"id"`
	OrganizationID string  `db:"organization_id"`
	Name           string  `db:"name"`
	Description    string  `db:"description"`
	FTMEntity      *string `db:"ftm_entity"`
}

const (
	TableDataModelTables  = "data_model_tables"
	TableDataModelFields  = "data_model_fields"
	TableDataModelAliases = "data_model_field_aliases"
)

var (
	SelectDataModelTableColumns      = utils.ColumnList[DbDataModelTable]()
	SelectDataModelFieldAliasColumns = utils.ColumnList[DbDataModelFieldAlias]()
)

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
	}, nil
}

type DbDataModelTableJoinField struct {
	TableID          string  `db:"data_model_tables.id"`
	OrganizationID   string  `db:"data_model_tables.organization_id"`
	TableName        string  `db:"data_model_tables.name"`
	TableDescription string  `db:"data_model_tables.description"`
	TableFTMEntity   *string `db:"data_model_tables.ftm_entity"`
	FieldID          string  `db:"data_model_fields.id"`
	PhysicalName     string  `db:"data_model_fields.name"`
	FieldType        string  `db:"data_model_fields.type"`
	FieldNullable    bool    `db:"data_model_fields.nullable"`
	FieldDescription string  `db:"data_model_fields.description"`
	FieldIsEnum      bool    `db:"data_model_fields.is_enum"`
	FieldFTMProperty *string `db:"data_model_fields.ftm_property"`
	FieldArchived    bool    `db:"data_model_fields.archived"`

	FieldName string   `db:"-"`
	Aliases   []string `db:"-"`
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

type DbDataModelFieldAlias struct {
	Id        uuid.UUID `db:"id"`
	TableId   uuid.UUID `db:"table_id"`
	FieldId   uuid.UUID `db:"field_id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

func AdaptDataModelFieldAlias(db DbDataModelFieldAlias) (models.DataModelFieldAlias, error) {
	return models.DataModelFieldAlias{
		Id:        db.Id,
		TableId:   db.TableId,
		FieldId:   db.FieldId,
		Name:      db.Name,
		CreatedAt: db.CreatedAt,
	}, nil
}
