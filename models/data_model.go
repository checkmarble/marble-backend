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
)

func (dm DataModel) Copy() DataModel {
	tables := make(map[TableName]Table)
	for k, v := range dm.Tables {
		tables[k] = v.Copy()
	}
	return DataModel{
		Version: dm.Version,
		Tables:  tables,
	}
}

// ///////////////////////////////
// Data Model table
// ///////////////////////////////

type Table struct {
	ID            string
	Name          TableName
	Description   string
	Fields        map[string]Field
	LinksToSingle map[string]LinkToSingle
}

func (t Table) Copy() Table {
	fields := make(map[string]Field)
	for k, v := range t.Fields {
		fields[k] = v
	}
	links := make(map[string]LinkToSingle)
	for k, v := range t.LinksToSingle {
		links[k] = v
	}
	out := t
	out.Fields = fields
	out.LinksToSingle = links
	return out
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
	Name              string
	Nullable          bool
	TableId           string
	Values            []any
	UnicityConstraint UnicityConstraint
}

type FieldMetadata struct {
	ID          string
	DataType    DataType
	Description string
	IsEnum      bool
	Name        string
	Nullable    bool
	TableId     string
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
	Name        string
	Description string
	DataType    DataType
	Nullable    bool
	IsEnum      bool
	IsUnique    bool
}

type UpdateFieldInput struct {
	Description *string
	IsEnum      *bool
	IsUnique    *bool
}

// ///////////////////////////////
// Data Model Link
// ///////////////////////////////
type LinkToSingle struct {
	Name            string
	LinkedTableName TableName
	ParentFieldName string
	ChildTableName  TableName
	ChildFieldName  string
}

type DataModelLinkCreateInput struct {
	OrganizationID string
	Name           string
	ParentTableID  string
	ParentFieldID  string
	ChildTableID   string
	ChildFieldID   string
}
