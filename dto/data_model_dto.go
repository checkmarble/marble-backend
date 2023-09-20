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
	Description string `json:"description"`
	DataType    string `json:"data_type"`
	Nullable    bool   `json:"nullable"`
}

type Table struct {
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

type PostDataModel struct {
	Body *struct {
		DataModel DataModel `json:"data_model"`
	} `in:"body=json"`
}

type PostCreateTable struct {
	Body *struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Nullable    bool   `json:"nullable"`
	} `in:"body=json"`
}

type PostCreateField struct {
	Body *struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Nullable    bool   `json:"nullable"`
	} `in:"body=json"`
}

func AdaptTableDto(table models.Table) Table {
	return Table{
		Name: string(table.Name),
		Fields: utils.MapMap(table.Fields, func(field models.Field) Field {
			return Field{
				DataType:    field.DataType.String(),
				Nullable:    field.Nullable,
				Description: field.Description,
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

func AdaptDataModel(dataModelDto DataModel) models.DataModel {
	return models.DataModel{
		Version: dataModelDto.Version,
		Status:  models.StatusFrom(dataModelDto.Status),
		Tables: utils.MapMap(dataModelDto.Tables, func(tableDto Table) models.Table {
			return models.Table{
				Name:        models.TableName(tableDto.Name),
				Description: tableDto.Description,
				Fields: utils.MapMap(tableDto.Fields, func(fieldDto Field) models.Field {
					return models.Field{
						Description: fieldDto.Description,
						DataType:    models.DataTypeFrom(fieldDto.DataType),
						Nullable:    fieldDto.Nullable,
					}
				}),
				LinksToSingle: utils.MapMap(tableDto.LinksToSingle, func(linkDto LinkToSingle) models.LinkToSingle {
					return models.LinkToSingle{
						LinkedTableName: linkDto.LinkedTableName,
						ParentFieldName: linkDto.ParentFieldName,
						ChildFieldName:  linkDto.ChildFieldName,
					}
				}),
			}
		}),
	}
}
