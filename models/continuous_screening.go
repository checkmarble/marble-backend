package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ContinuousScreeningTriggerType int

const (
	ContinuousScreeningTriggerTypeObjectAdded ContinuousScreeningTriggerType = iota
	ContinuousScreeningTriggerTypeObjectUpdated
	ContinuousScreeningTriggerTypeDatasetUpdated
	ContinuousScreeningTriggerTypeUnknown
)

func ContinuousScreeningTriggerTypeFrom(s string) ContinuousScreeningTriggerType {
	switch s {
	case "object_added":
		return ContinuousScreeningTriggerTypeObjectAdded
	case "object_updated":
		return ContinuousScreeningTriggerTypeObjectUpdated
	case "dataset_updated":
		return ContinuousScreeningTriggerTypeDatasetUpdated
	}

	return ContinuousScreeningTriggerTypeUnknown
}

func (stt ContinuousScreeningTriggerType) String() string {
	switch stt {
	case ContinuousScreeningTriggerTypeObjectAdded:
		return "object_added"
	case ContinuousScreeningTriggerTypeObjectUpdated:
		return "object_updated"
	case ContinuousScreeningTriggerTypeDatasetUpdated:
		return "dataset_updated"
	}

	return "unknown"
}

type CreateContinuousScreeningObject struct {
	ObjectType     string
	ConfigStableId uuid.UUID
	ObjectId       *string
	ObjectPayload  *json.RawMessage
}

type DeleteContinuousScreeningObject struct {
	ObjectType     string
	ObjectId       string
	ConfigStableId uuid.UUID
}

type ContinuousScreening struct {
	Id                                uuid.UUID
	OrgId                             uuid.UUID
	ContinuousScreeningConfigId       uuid.UUID
	ContinuousScreeningConfigStableId uuid.UUID
	CaseId                            *uuid.UUID
	ObjectType                        string
	ObjectId                          string
	ObjectInternalId                  uuid.UUID
	Status                            ScreeningStatus
	TriggerType                       ContinuousScreeningTriggerType
	SearchInput                       json.RawMessage
	IsPartial                         bool
	NumberOfMatches                   int

	CreatedAt time.Time
	UpdatedAt time.Time
}

type ContinuousScreeningMatch struct {
	Id                    uuid.UUID
	ContinuousScreeningId uuid.UUID
	OpenSanctionEntityId  string
	Status                ScreeningMatchStatus
	Payload               json.RawMessage
	ReviewedBy            *uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
}

type ContinuousScreeningWithMatches struct {
	ContinuousScreening

	Matches []ContinuousScreeningMatch
}

const ContinuousScreeningSortingCreatedAt SortingField = SortingFieldCreatedAt

type ContinuousScreeningMonitoredObject struct {
	Id             uuid.UUID
	ObjectType     string
	ObjectId       string
	ConfigStableId uuid.UUID
	CreatedAt      time.Time
}

type ContinuousScreeningDataModelMapping struct {
	Entity     string
	Properties map[string]string
}

type ListMonitoredObjectsFilters struct {
	ObjectTypes     []string    // Optional: filter by object types
	ObjectIds       []string    // Optional: filter by object IDs
	ConfigStableIds []uuid.UUID // Optional: filter by config stable IDs
	StartDate       time.Time   // Optional: filter objects created on/after this date
	EndDate         time.Time   // Optional: filter objects created on/before this date
}
