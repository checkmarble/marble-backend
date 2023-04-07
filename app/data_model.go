package app

import "context"

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
	Tables  map[string]Table `json:"tables"`
}

type Table struct {
	Name          string                  `json:"name"`
	Fields        map[string]Field        `json:"fields"`
	LinksToSingle map[string]LinkToSingle `json:"linksToSingle"`
}

type Field struct {
	DataType DataType `json:"dataType"`
}

type LinkToSingle struct {
	LinkedTableName string `json:"linkedTableName"`
	ParentFieldName string `json:"parentFieldName"`
	ChildFieldName  string `json:"childFieldName"`
}

func (app *App) GetDataModel(ctx context.Context, orgID string) (DataModel, error) {
	dataModel, err := app.repository.GetDataModel(ctx, orgID)
	if err != nil {
		return DataModel{}, err
	}
	return dataModel, nil
}

///////////////////////////////
// Data Access
///////////////////////////////

func (dm DataModel) FieldAt(rootName string, path []string) Field {
	currentRoot := dm.Tables[rootName]

	if len(path) == 1 {
		return currentRoot.Fields[path[0]]
	}

	return dm.FieldAt(currentRoot.LinksToSingle[path[0]].LinkedTableName, path[1:])

}

func (dm DataModel) FieldValueAtFromPayload(payload Payload, path []string) interface{} {

	// Value is found
	if len(path) == 1 {
		return payload.Data[path[0]]
	}

	// Value needs to be derived
	// TODO

	return nil
}
