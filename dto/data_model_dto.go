package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type LinkToSingle struct {
	LinkedTableName models.TableName `json:"linked_table_name"`
	ParentFieldName models.FieldName `json:"parent_field_name"`
	ChildFieldName  models.FieldName `json:"child_field_name"`
}

type Field struct {
	ID                string `json:"id,omitempty"`
	DataType          string `json:"data_type"`
	Description       string `json:"description"`
	IsEnum            bool   `json:"is_enum"`
	Nullable          bool   `json:"nullable"`
	Values            []any  `json:"values,omitempty"`
	UnicityConstraint string `json:"unicity_constraint"`
}

type Table struct {
	ID            string                           `json:"id,omitempty"`
	Name          string                           `json:"name"`
	Description   string                           `json:"description"`
	Fields        map[models.FieldName]Field       `json:"fields"`
	LinksToSingle map[models.LinkName]LinkToSingle `json:"links_to_single,omitempty"`
}

type DataModel struct {
	Version string                     `json:"version"`
	Tables  map[models.TableName]Table `json:"tables"`
}

type PostDataModel struct {
	Body *struct {
		DataModel DataModel `json:"data_model"`
	} `in:"body=json;required"`
}

type PostCreateTable struct {
	Body *struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `in:"body=json"`
}

type PostCreateField struct {
	Body *struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Nullable    bool   `json:"nullable"`
		IsEnum      bool   `json:"is_enum"`
	} `in:"body=json"`
}

type PostCreateLink struct {
	Body *struct {
		Name          string `json:"name"`
		ParentTableID string `json:"parent_table_id"`
		ParentFieldID string `json:"parent_field_id"`
		ChildTableID  string `json:"child_table_id"`
		ChildFieldID  string `json:"child_field_id"`
	} `in:"body=json"`
}

type PostToggleIsEnum struct {
	Body *struct {
		FieldID string `json:"field_id"`
	} `in:"body=json"`
}

func AdaptTableDto(table models.Table) Table {
	return Table{
		Name: string(table.Name),
		ID:   table.ID,
		Fields: pure_utils.MapValues(table.Fields, func(field models.Field) Field {
			return Field{
				ID:                field.ID,
				DataType:          field.DataType.String(),
				Description:       field.Description,
				IsEnum:            field.IsEnum,
				Nullable:          field.Nullable,
				Values:            field.Values,
				UnicityConstraint: field.UnicityConstraint.String(),
			}
		}),
		LinksToSingle: pure_utils.MapValues(table.LinksToSingle, func(
			linkToSingle models.LinkToSingle,
		) LinkToSingle {
			return LinkToSingle{
				LinkedTableName: linkToSingle.LinkedTableName,
				ParentFieldName: linkToSingle.ParentFieldName,
				ChildFieldName:  linkToSingle.ChildFieldName,
			}
		}),
		Description: table.Description,
	}
}

func AdaptDataModelDto(dataModel models.DataModel) DataModel {
	return DataModel{
		Version: dataModel.Version,
		Tables:  pure_utils.MapValues(dataModel.Tables, AdaptTableDto),
	}
}
