package models

// /////////////////////////////
// Data types
// /////////////////////////////
type DataType int

const (
	Bool DataType = iota
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
	Version string
	Status  Status
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
	Name          TableName                 `json:"name"`
	Fields        map[FieldName]Field       `json:"fields"`
	LinksToSingle map[LinkName]LinkToSingle `json:"linksToSingle"`
}

type Field struct {
	Name     FieldName `json:"name"`
	DataType DataType  `json:"dataType"`
	Nullable bool      `json:"nullable"`
}

type LinkToSingle struct {
	LinkedTableName TableName `json:"linkedTableName"`
	ParentFieldName FieldName `json:"parentFieldName"`
	ChildFieldName  FieldName `json:"childFieldName"`
}
