package models

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

type Table struct {
	ID            string
	Name          TableName
	Description   string
	Fields        map[FieldName]Field
	LinksToSingle map[LinkName]LinkToSingle
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

type Field struct {
	ID                string
	DataType          DataType
	Description       string
	IsEnum            bool
	Nullable          bool
	TableId           string
	Values            []any
	UnicityConstraint UnicityConstraint
}

type LinkToSingle struct {
	LinkedTableName TableName
	ParentFieldName FieldName
	ChildFieldName  FieldName
}

type DataModelTable struct {
	ID             string
	OrganizationID string
	Name           string
	Description    string
}

type DataModelField struct {
	Name        string
	Description string
	Type        string
	Nullable    bool
	IsEnum      bool
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
	ID             string
	OrganizationID string
	Name           LinkName
	ParentTableID  string
	ParentTable    TableName
	ParentFieldID  string
	ParentField    FieldName
	ChildTableID   string
	ChildTable     TableName
	ChildFieldID   string
	ChildField     FieldName
}
