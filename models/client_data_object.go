package models

import (
	"time"
)

type PivotObject struct {
	PivotObjectId     string
	PivotValue        string
	PivotId           string
	PivotType         PivotType
	PivotObjectName   string
	PivotFieldName    string
	IsIngested        bool
	PivotObjectData   ClientObjectDetail
	NumberOfDecisions int
}

// PivotType corresponds to the type of entity that is materialized by a pivot value.
// A pivot can be a concrete object if it identifies a unique ingested object:
//   - a pivot defined by a (seris of) link from the base table identifies the object at the end of the links
//   - a pivot defined as a unique field (object_id or other) on a table, identifies that object
//   - conversely, a pivot defined by a "grouping" field on a table where many rows may share that value (e.g. "transactions.account_id")
//     allows to group decisions, snooze rules etc, but does not identify a concrete object that can be ingested.
//
// Most pivot definitions should be of type "object", but we have to support the other case for backward compatibility.
type PivotType int

const (
	PivotTypeUnknown PivotType = iota
	PivotTypeObject
	PivotTypeField
)

func (p PivotType) String() string {
	switch p {
	case PivotTypeObject:
		return "object"
	case PivotTypeField:
		return "field"
	default:
		return "unknown"
	}
}

func PivotTypeFromString(s string) PivotType {
	switch s {
	case "object":
		return PivotTypeObject
	case "field":
		return PivotTypeField
	default:
		return PivotTypeUnknown
	}
}

type ClientObjectDetail struct {
	Metadata       ClientObjectMetadata
	Data           map[string]any
	RelatedObjects []RelatedObject
	Annotations    GroupedEntityAnnotations
}

type RelatedObject struct {
	LinkName string
	Detail   ClientObjectDetail
}

type ClientObjectMetadata struct {
	ValidFrom  time.Time
	ObjectType string
}

type StringOrNumber struct {
	StringValue *string
	FloatValue  *float64
}

type ClientDataListPagination struct {
	NextCursorId *string
	HasNextPage  bool
}

type ClientDataListRequestBody struct {
	ExplorationOptions ExplorationOptions
	CursorId           *string
	Limit              int
}

type ExplorationOptions struct {
	SourceTableName   string
	FilterFieldName   string
	FilterFieldValue  StringOrNumber
	OrderingFieldName string
}
