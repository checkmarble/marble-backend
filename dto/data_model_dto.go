package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type LinkToSingle struct {
	LinkedTableName models.TableName `json:"linked_table_name"`
	ParentFieldName models.FieldName `json:"parent_field_name"`
	ChildFieldName  models.FieldName `json:"child_field_name"`
}

type Field struct {
	ID          string   `json:"id,omitempty"`
	Description string   `json:"description"`
	DataType    string   `json:"data_type"`
	Nullable    bool     `json:"nullable"`
	IsEnum      bool     `json:"is_enum"`
	Values      []string `json:"values,omitempty"`
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
	Status  string                     `json:"status"`
	Tables  map[models.TableName]Table `json:"tables"`
}

func AdaptTableDto(table models.Table) Table {
	return Table{
		Name: string(table.Name),
		ID:   table.ID,
		Fields: utils.MapMap(table.Fields, func(field models.Field) Field {
			return Field{
				ID:          field.ID,
				DataType:    field.DataType.String(),
				Nullable:    field.Nullable,
				Description: field.Description,
				IsEnum:      field.IsEnum,
				Values:      field.Values,
			}
		}),
		LinksToSingle: utils.MapMap(table.LinksToSingle, func(linkToSingle models.LinkToSingle) LinkToSingle {
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
		Status:  dataModel.Status.String(),
		Tables:  utils.MapMap(dataModel.Tables, AdaptTableDto),
	}
}
