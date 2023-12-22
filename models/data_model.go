package models

import "fmt"

// /////////////////////////////
// Data types
// /////////////////////////////
type DataType int

// Careful: DataType is serialized in database, it's also a dto
const (
	UnknownDataType DataType = iota - 1
	Bool
	Int
	Float
	String
	Timestamp
)

func (d DataType) String() string {
	switch d {
	case Bool:
		return "Bool"
	case Int:
		return "Int"
	case Float:
		return "Float"
	case String:
		return "String"
	case Timestamp:
		return "Timestamp"
	}
	return "unknown"
}

func (d DataType) ToPostgresType() string {
	switch d {
	case Int:
		return "INTEGER"
	case String:
		return "TEXT"
	case Timestamp:
		return "TIMESTAMP WITH TIME ZONE"
	case Float:
		return "FLOAT"
	case Bool:
		return "BOOLEAN"
	default:
		panic(fmt.Errorf("unknown data type: %v", d))
	}
}

func DataTypeFrom(s string) DataType {
	switch s {
	case "Bool":
		return Bool
	case "Int":
		return Int
	case "Float":
		return Float
	case "String":
		return String
	case "Timestamp":
		return Timestamp
	}
	return UnknownDataType
}

// /////////////////////////////
// Status
// /////////////////////////////
type Status int

const (
	Validated Status = iota
	Live
	Deprecated
)

// Provide a string value for each status
func (o Status) String() string {
	switch o {
	case Validated:
		return "validated"
	case Live:
		return "live"
	case Deprecated:
		return "deprecated"
	}
	return "deprecated"
}

// Provide an Status from a string value
func StatusFrom(s string) Status {
	switch s {
	case "validated":
		return Validated
	case "live":
		return Live
	case "deprecated":
		return Deprecated
	}
	return Deprecated
}

///////////////////////////////
// Data Model
///////////////////////////////

type DataModel struct {
	Version string              `json:"version"`
	Status  Status              `json:"status"`
	Tables  map[TableName]Table `json:"tables"`
}

type (
	TableName string
	FieldName string
	LinkName  string
)

func ToLinkNames(arr []string) []LinkName {
	var result []LinkName
	for _, s := range arr {
		result = append(result, LinkName(s))
	}
	return result
}

type Table struct {
	ID            string                    `json:"id,omitempty"`
	Name          TableName                 `json:"name"`
	Description   string                    `json:"description"`
	Fields        map[FieldName]Field       `json:"fields"`
	LinksToSingle map[LinkName]LinkToSingle `json:"linksToSingle"`
}

func ColumnNames(table Table) []string {
	columnNames := make([]string, len(table.Fields))
	i := 0
	for fieldName := range table.Fields {
		columnNames[i] = string(fieldName)
		i++
	}
	return columnNames
}

type Field struct {
	ID          string   `json:"id,omitempty"`
	Description string   `json:"description"`
	DataType    DataType `json:"dataType"`
	Nullable    bool     `json:"nullable"`
	IsEnum      bool     `json:"is_enum"`
	Values      []any    `json:"values,omitempty"`
}

type LinkToSingle struct {
	LinkedTableName TableName `json:"linkedTableName"`
	ParentFieldName FieldName `json:"parentFieldName"`
	ChildFieldName  FieldName `json:"childFieldName"`
}

type DataModelTable struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
}

type DataModelField struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"dataType"`
	Nullable    bool   `json:"nullable"`
	IsEnum      bool   `json:"is_enum"`
}

type UpdateDataModelFieldInput struct {
	Description *string
	IsEnum      *bool
}

type DataModelTableField struct {
	TableID          string
	OrganizationID   string
	TableName        string
	TableDescription string
	FieldID          string
	FieldName        string
	FieldType        string
	FieldNullable    bool
	FieldDescription string
	FieldIsEnum      bool
}

type DataModelLink struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           LinkName  `json:"name"`
	ParentTableID  string    `json:"parent_table_id"`
	ParentTable    TableName `json:"parent_table"`
	ParentFieldID  string    `json:"parent_field_id"`
	ParentField    FieldName `json:"parent_field"`
	ChildTableID   string    `json:"child_table_id"`
	ChildTable     TableName `json:"child_table"`
	ChildFieldID   string    `json:"child_field_id"`
	ChildField     FieldName `json:"child_field"`
}
