package models

import "slices"

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
	Tables  map[string]Table
}

func (dm DataModel) Copy() DataModel {
	tables := make(map[string]Table)
	for k, v := range dm.Tables {
		tables[k] = v.Copy()
	}
	return DataModel{
		Version: dm.Version,
		Tables:  tables,
	}
}

func (dm DataModel) AllLinksAsMap() map[string]LinkToSingle {
	links := make(map[string]LinkToSingle, 100)
	for _, table := range dm.Tables {
		for _, link := range table.LinksToSingle {
			links[link.Id] = link
		}
	}
	return links
}

func (dm DataModel) AllTablesAsMap() map[string]Table {
	tables := make(map[string]Table, 100)
	for _, table := range dm.Tables {
		tables[table.ID] = table
	}
	return tables
}

func (dm DataModel) AllFieldsAsMap() map[string]Field {
	fields := make(map[string]Field, 100)
	for _, table := range dm.Tables {
		for _, field := range table.Fields {
			fields[field.ID] = field
		}
	}
	return fields
}

// ///////////////////////////////
// Data Model table
// ///////////////////////////////

type Table struct {
	ID                string
	Name              string
	Description       string
	Fields            map[string]Field
	LinksToSingle     map[string]LinkToSingle
	NavigationOptions []NavigationOption
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
		columnNames[i] = fieldName
		i++
	}
	slices.Sort(columnNames)
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

type EnumValues map[string]map[any]struct{}

// CollectEnumValues mutates the EnumValues object to collect all the enum values from the payload
func (enumValues EnumValues) CollectEnumValues(payload ClientObject) {
	for fieldName := range enumValues {
		value := payload.Data[fieldName]
		if value != nil && value != "" {
			enumValues[fieldName][value] = struct{}{}
		}
	}
}

// ///////////////////////////////
// Data Model Link
// ///////////////////////////////
type LinkToSingle struct {
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

type DataModelLinkCreateInput struct {
	OrganizationID string
	Name           string
	ParentTableID  string
	ParentFieldID  string
	ChildTableID   string
	ChildFieldID   string
}

type DataModelObject struct {
	Data     map[string]any
	Metadata map[string]any
}

// ///////////////////////////////
// Navigation options - AKA how we can explore client data objects in a "one to many" way
// ///////////////////////////////

type NavigationOption struct {
	ParentFieldName   string
	ParentFieldId     string
	ChildTableName    string
	ChildTableId      string
	ChildFieldName    string
	ChildFieldId      string
	OrderingFieldName string
	OrderingFieldId   string
	Status            IndexStatus
}
