package models

// ///////////////////////////////
// Data Type
// ///////////////////////////////
type DataType int

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

///////////////////////////////
// Data Model
///////////////////////////////

type DataModel struct {
	Version string
	Tables  map[TableName]Table
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

// ///////////////////////////////
// Data Model table
// ///////////////////////////////

type Table struct {
	ID            string
	Name          TableName
	Description   string
	Fields        map[FieldName]Field
	LinksToSingle map[LinkName]LinkToSingle
}

type TableMetadata struct {
	ID             string
	Description    string
	Name           string
	OrganizationID string
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

// ///////////////////////////////
// Data Type
// ///////////////////////////////

type Field struct {
	ID                string
	DataType          DataType
	Description       string
	IsEnum            bool
	Name              FieldName
	Nullable          bool
	TableId           string
	Values            []any
	UnicityConstraint UnicityConstraint
}

type UnicityConstraint int

const (
	NoUnicityConstraint UnicityConstraint = iota
	ActiveUniqueConstraint
	PendingUniqueConstraint
)

func (u UnicityConstraint) String() string {
	switch u {
	case NoUnicityConstraint:
		return "no_unicity_constraint"
	case ActiveUniqueConstraint:
		return "active_unique_constraint"
	case PendingUniqueConstraint:
		return "pending_unique_constraint"
	}
	return "unknown"
}

func UnicityConstraintFromString(s string) UnicityConstraint {
	switch s {
	case "no_unicity_constraint":
		return NoUnicityConstraint
	case "active_unique_constraint":
		return ActiveUniqueConstraint
	case "pending_unique_constraint":
		return PendingUniqueConstraint
	}
	return NoUnicityConstraint
}

type CreateFieldInput struct {
	TableId     string
	Name        FieldName
	Description string
	DataType    DataType
	Nullable    bool
	IsEnum      bool
}

type UpdateFieldInput struct {
	Description *string
	IsEnum      *bool
}

// ///////////////////////////////
// Data Model Link
// ///////////////////////////////
type LinkToSingle struct {
	Name            LinkName
	LinkedTableName TableName
	ParentFieldName FieldName
	ChildTableName  TableName
	ChildFieldName  FieldName
}

type DataModelLinkCreateInput struct {
	OrganizationID string
	Name           LinkName
	ParentTableID  string
	ParentFieldID  string
	ChildTableID   string
	ChildFieldID   string
}
