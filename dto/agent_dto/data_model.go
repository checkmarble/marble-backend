package agent_dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type LinkToSingle struct {
	ParentTableName string `json:"parent_table_name"`
	ParentFieldName string `json:"parent_field_name"`
	ChildTableName  string `json:"child_table_name"`
	ChildFieldName  string `json:"child_field_name"`
}

type Field struct {
	DataType    string   `json:"data_type"`
	Description string   `json:"description"`
	IsEnum      bool     `json:"is_enum"`
	Name        string   `json:"name"`
	EnumSample  []any    `json:"enum_sample,omitempty"`
	Histogram   []string `json:"histogram,omitempty"`
	Format      string   `json:"format,omitempty"`
	MaxLength   int      `json:"max_length,omitempty"`
}

type Table struct {
	Name          string                  `json:"name"`
	Description   string                  `json:"description"`
	Fields        map[string]Field        `json:"fields"`
	LinksToSingle map[string]LinkToSingle `json:"links_to_single,omitempty"`
	Alias         string                  `json:"alias"`
	SemanticType  string                  `json:"semantic_type"`
}

type DataModel map[string]Table

func adaptTableDto(table models.Table) Table {
	return Table{
		Name:          table.Name,
		Fields:        pure_utils.MapValues(table.Fields, adaptDataModelField),
		LinksToSingle: pure_utils.MapValues(table.LinksToSingle, adaptDataModelLink),
		Description:   table.Description,
		Alias:         table.Alias,
		SemanticType:  string(table.SemanticType),
	}
}

func adaptDataModelField(field models.Field) Field {
	return Field{
		DataType:    field.DataType.String(),
		Description: field.Description,
		IsEnum:      field.IsEnum,
		Name:        field.Name,
		EnumSample:  field.Values,
		Histogram:   field.FieldStatistics.Histogram,
		Format:      field.FieldStatistics.Format,
		MaxLength:   field.FieldStatistics.MaxLength,
	}
}

func adaptDataModelLink(linkToSingle models.LinkToSingle) LinkToSingle {
	return LinkToSingle{
		ParentTableName: linkToSingle.ParentTableName,
		ParentFieldName: linkToSingle.ParentFieldName,
		ChildTableName:  linkToSingle.ChildTableName,
		ChildFieldName:  linkToSingle.ChildFieldName,
	}
}

func AdaptDataModelDto(dataModel models.DataModel) DataModel {
	return pure_utils.MapValues(dataModel.Tables, adaptTableDto)
}
