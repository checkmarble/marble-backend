package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type InsertContinuousScreeningObject struct {
	ObjectType     string
	ConfigStableId uuid.UUID
	ObjectId       *string
	ObjectPayload  *json.RawMessage
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
