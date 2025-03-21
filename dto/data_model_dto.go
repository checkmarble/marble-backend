package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type LinkToSingle struct {
	Id              string `json:"id"`
	ParentTableName string `json:"parent_table_name"`
	ParentTableId   string `json:"parent_table_id"`
	ParentFieldName string `json:"parent_field_name"`
	ParentFieldId   string `json:"parent_field_id"`
	ChildTableName  string `json:"child_table_name"`
	ChildTableId    string `json:"child_table_id"`
	ChildFieldName  string `json:"child_field_name"`
	ChildFieldId    string `json:"child_field_id"`
}

type Field struct {
	ID                string `json:"id"`
	DataType          string `json:"data_type"`
	Description       string `json:"description"`
	IsEnum            bool   `json:"is_enum"`
	Name              string `json:"name"`
	Nullable          bool   `json:"nullable"`
	TableId           string `json:"table_id"`
	Values            []any  `json:"values,omitempty"`
	UnicityConstraint string `json:"unicity_constraint"`
}

type Table struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description"`
	Fields        map[string]Field        `json:"fields"`
	LinksToSingle map[string]LinkToSingle `json:"links_to_single,omitempty"`
}

type DataModel struct {
	Tables map[string]Table `json:"tables"`
}

func AdaptTableDto(table models.Table) Table {
	return Table{
		Name:          table.Name,
		ID:            table.ID,
		Fields:        pure_utils.MapValues(table.Fields, adaptDataModelField),
		LinksToSingle: pure_utils.MapValues(table.LinksToSingle, adaptDataModelLink),
		Description:   table.Description,
	}
}

func adaptDataModelField(field models.Field) Field {
	return Field{
		ID:                field.ID,
		DataType:          field.DataType.String(),
		Description:       field.Description,
		IsEnum:            field.IsEnum,
		Name:              field.Name,
		Nullable:          field.Nullable,
		TableId:           field.TableId,
		Values:            field.Values,
		UnicityConstraint: field.UnicityConstraint.String(),
	}
}

func adaptDataModelLink(linkToSingle models.LinkToSingle) LinkToSingle {
	return LinkToSingle{
		Id:              linkToSingle.Id,
		ParentTableName: linkToSingle.ParentTableName,
		ParentTableId:   linkToSingle.ParentTableId,
		ParentFieldName: linkToSingle.ParentFieldName,
		ParentFieldId:   linkToSingle.ParentFieldId,
		ChildTableName:  linkToSingle.ChildTableName,
		ChildTableId:    linkToSingle.ChildTableId,
		ChildFieldName:  linkToSingle.ChildFieldName,
		ChildFieldId:    linkToSingle.ChildFieldId,
	}
}

func AdaptDataModelDto(dataModel models.DataModel) DataModel {
	return DataModel{
		Tables: pure_utils.MapValues(dataModel.Tables, AdaptTableDto),
	}
}

type CreateTableInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateTableInput struct {
	Description string `json:"description"`
}

type CreateLinkInput struct {
	Name          string `json:"name"`
	ParentTableId string `json:"parent_table_id"`
	ParentFieldId string `json:"parent_field_id"`
	ChildTableId  string `json:"child_table_id"`
	ChildFieldId  string `json:"child_field_id"`
}

type UpdateFieldInput struct {
	Description *string `json:"description"`
	IsEnum      *bool   `json:"is_enum"`
	IsUnique    *bool   `json:"is_unique"`
}

type CreateFieldInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Nullable    bool   `json:"nullable"`
	IsEnum      bool   `json:"is_enum"`
	IsUnique    bool   `json:"is_unique"`
}

type DataModelObject struct {
	Data     map[string]any `json:"data"`
	Metadata map[string]any `json:"metadata"`
}
