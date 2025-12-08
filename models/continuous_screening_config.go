package models

import (
	"time"

	"github.com/google/uuid"
)

// Configuration for continuous screening for an organization.
// Defines a set of datasets that are used for the monitoring.
type ContinuousScreeningConfig struct {
	Id          uuid.UUID
	StableId    uuid.UUID
	OrgId       uuid.UUID
	InboxId     uuid.UUID
	Name        string
	Description string
	ObjectTypes []string
	Algorithm   string
	// Dataset that are used for the monitoring
	Datasets []string

	// Threshold used in matching score, between 0 and 100
	MatchThreshold int

	MatchLimit int
	Enabled    bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

type ContinuousScreeningConfigParameters struct {
	MatchThreshold int
	MatchLimit     int
	Datasets       []string
}

type ContinuousScreeningMappingField struct {
	ObjectFieldId uuid.UUID
	FtmProperty   FollowTheMoneyProperty
}

type ContinuousScreeningMappingConfig struct {
	ObjectType          string
	FtmEntity           FollowTheMoneyEntity
	ObjectFieldMappings []ContinuousScreeningMappingField
}

type CreateContinuousScreeningConfig struct {
	OrgId          uuid.UUID
	StableId       uuid.UUID
	InboxId        *uuid.UUID
	InboxName      *string
	Name           string
	Description    string
	Algorithm      string
	Datasets       []string
	MatchThreshold int
	MatchLimit     int
	ObjectTypes    []string
	MappingConfigs []ContinuousScreeningMappingConfig
}

type UpdateContinuousScreeningConfig struct {
	InboxId        *uuid.UUID
	InboxName      *string
	Name           *string
	Description    *string
	Algorithm      *string
	ObjectTypes    *[]string
	Datasets       *[]string
	MatchThreshold *int
	MatchLimit     *int
	Enabled        *bool
	MappingConfigs []ContinuousScreeningMappingConfig
}
