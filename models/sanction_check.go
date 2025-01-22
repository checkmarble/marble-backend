package models

import (
	"encoding/json"
	"time"
)

type OpenSanctionCheckFilter map[string][]string

type OpenSanctionsQuery struct {
	Queries   OpenSanctionCheckFilter
	OrgConfig OrganizationOpenSanctionsConfig
}

type SanctionCheck struct {
	Id          string
	DecisionId  string
	Status      string
	Query       json.RawMessage
	OrgConfig   OrganizationOpenSanctionsConfig
	IsManual    bool
	RequestedBy *string
	Partial     bool
	Count       int
	Matches     []SanctionCheckMatch
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SanctionCheckMatch struct {
	Id              string
	SanctionCheckId string
	EntityId        string
	Status          string
	QueryIds        []string
	Payload         []byte
	ReviewedBy      *string
	CommentCount    int
}

type SanctionCheckMatchUpdate struct {
	ReviewerId UserId
	Status     string
}

type SanctionCheckMatchComment struct {
	Id          string
	MatchId     string
	CommenterId UserId
	Comment     string
	CreatedAt   time.Time
}
