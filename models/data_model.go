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

func (t Table) GetFieldById(fieldId string) (Field, bool) {
	for _, field := range t.Fields {
		if field.ID == fieldId {
			return field, true
		}
	}
	return Field{}, false
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

// Utility methods on data model

func (d DataModel) AddUnicityConstraintStatusToDataModel(uniqueIndexes []UnicityIndex) DataModel {
	dm := d.Copy()
	for _, index := range uniqueIndexes {
		// here we only care about single fields with a unicity constraint
		if len(index.Fields) != 1 {
			continue
		}
		table, ok := dm.Tables[index.TableName]
		if !ok {
			continue
		}
		field, ok := table.Fields[index.Fields[0]]
		if !ok {
			continue
		}

		if field.Name == index.Fields[0] {
			if index.CreationInProcess && field.UnicityConstraint != ActiveUniqueConstraint {
				field.UnicityConstraint = PendingUniqueConstraint
			} else {
				field.UnicityConstraint = ActiveUniqueConstraint
			}
			// cannot directly modify the struct field in the map, so we need to reassign it
			dm.Tables[index.TableName].Fields[index.Fields[0]] = field
		}
	}
	return dm
}

func (d DataModel) AddNavigationOptionsToDataModel(indexes []ConcreteIndex, pivots []Pivot) DataModel {
	dm := d.Copy()
	// navigation options are computed from the following heuristic:
	// - table A has a link to table B (through A.a -> B.b) and there exists an index table A on (a, some_timestamp_field)
	// - table A has a pivot value defined that is a field of table A itself (e.g. "transactions.account_id"), and there exists an index table A on (a, some_timestamp_field)

	navigationOptions := make(map[string][]NavigationOption, len(d.Tables))

	for _, index := range indexes {
		if index.Type != IndexTypeNavigation {
			continue
		}

		childTable, ok := dm.Tables[index.TableName]
		if !ok {
			continue
		}

		if len(index.Indexed) < 2 {
			continue
		}
		fieldName := index.Indexed[0]
		field, ok := childTable.Fields[fieldName]
		if !ok {
			continue
		}

		childOrderingField, ok := childTable.Fields[index.Indexed[1]]
		if !ok {
			continue
		}

		var candidateLinksFromThisField []LinkToSingle
		for _, l := range childTable.LinksToSingle {
			if l.ChildFieldName == fieldName {
				candidateLinksFromThisField = append(candidateLinksFromThisField, l)
			}
		}

		for _, link := range candidateLinksFromThisField {
			// the parent table is the source table and the navigation option is the "reverse link", plus order.
			navOption := NavigationOption{
				SourceTableName:   link.ParentTableName,
				SourceTableId:     link.ParentTableId,
				SourceFieldName:   link.ParentFieldName,
				SourceFieldId:     link.ParentFieldId,
				TargetTableName:   link.ChildTableName,
				TargetTableId:     link.ChildTableId,
				FilterFieldName:   link.ChildFieldName,
				FilterFieldId:     link.ChildFieldId,
				OrderingFieldName: childOrderingField.Name,
				OrderingFieldId:   childOrderingField.ID,
				Status:            index.Status,
			}
			if _, ok := navigationOptions[link.ParentTableName]; !ok {
				navigationOptions[link.ParentTableName] = []NavigationOption{}
			}
			navigationOptions[link.ParentTableName] =
				append(navigationOptions[link.ParentTableName], navOption)
		}

		for _, pivot := range pivots {
			if pivot.BaseTable != index.TableName {
				continue
			}
			if pivot.Field != field.Name {
				continue
			}

			// the pivot table is the base table and the child and parent fields are the same
			navOption := NavigationOption{
				SourceTableName:   pivot.BaseTable,
				SourceTableId:     pivot.BaseTableId,
				SourceFieldName:   field.Name,
				SourceFieldId:     field.ID,
				TargetTableName:   pivot.PivotTable,
				TargetTableId:     pivot.PivotTableId,
				FilterFieldName:   field.Name,
				FilterFieldId:     field.ID,
				OrderingFieldName: childOrderingField.Name,
				OrderingFieldId:   childOrderingField.ID,
				Status:            index.Status,
			}
			if _, ok := navigationOptions[pivot.BaseTable]; !ok {
				navigationOptions[pivot.BaseTable] = []NavigationOption{}
			}
			navigationOptions[pivot.BaseTable] =
				append(navigationOptions[pivot.BaseTable], navOption)
		}
	}

	for tableName, table := range dm.Tables {
		if options, ok := navigationOptions[table.Name]; ok {
			t := table
			t.NavigationOptions = options
			dm.Tables[tableName] = t
		}
	}
	return dm
}

// Controls hwo the data model should be read. This allows us to specify what level of detail is needed on the returned data model, allowing to bypass some
// possibly expensive queries where only a partial data model is useful, while still factorizing the data model reading code in just one usecase method.
type DataModelReadOptions struct {
	// Controls whether the returned data model should include a sample of the enum values that have been seen, for fields that are flagged as enum.
	// Typically useful for the frontend, but not for internal usage in the backend.
	IncludeEnums bool

	// Controls whether the returned data model should include the navigation options between tables in the data model. Typically useful for the frontend
	// and some backend internal usage, but not everywhere (see ingested data reader usecase).
	IncludeNavigationOptions bool

	// Controls whether the returned data model should include information on fields marked as unique. If false, all fields will appear as having no unicity constraint.
	IncludeUnicityConstraints bool
}

// ///////////////////////////////
// Navigation options - AKA how we can explore client data objects in a "one to many" way
// ///////////////////////////////

type NavigationOption struct {
	// A navigation options starts from a table and field value
	SourceTableName string
	SourceTableId   string
	SourceFieldName string
	SourceFieldId   string

	// And goes to another table (may be the same), filtering on a field and ordering on another field
	TargetTableName   string
	TargetTableId     string
	FilterFieldName   string
	FilterFieldId     string
	OrderingFieldName string
	OrderingFieldId   string

	Status IndexStatus
}

type CreateNavigationOptionInput struct {
	SourceTableId   string
	SourceFieldId   string
	TargetTableId   string
	FilterFieldId   string
	OrderingFieldId string
}
