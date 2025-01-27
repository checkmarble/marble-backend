package models

import (
	"encoding/json"
	"time"
)

type SanctionCheckStatus int

const (
	SanctionStatusConfirmedHit SanctionCheckStatus = iota
	SanctionStatusNoHit
	SanctionStatusInReview
	SanctionStatusError
	SanctionStatusUnknown
)

func SanctionCheckStatusFrom(s string) SanctionCheckStatus {
	switch s {
	case "confirmed_hit":
		return SanctionStatusConfirmedHit
	case "no_hit":
		return SanctionStatusNoHit
	case "in_review":
		return SanctionStatusInReview
	case "error":
		return SanctionStatusError
	}

	return SanctionStatusUnknown
}

func (scs SanctionCheckStatus) String() string {
	switch scs {
	case SanctionStatusConfirmedHit:
		return "confirmed_hit"
	case SanctionStatusNoHit:
		return "no_hit"
	case SanctionStatusInReview:
		return "in_review"
	case SanctionStatusError:
		return "error"
	}

	return "unknown"
}

func (scs SanctionCheckStatus) IsFinalized() bool {
	return scs == SanctionStatusConfirmedHit || scs == SanctionStatusNoHit
}

type SanctionCheck struct {
	Id          string
	DecisionId  string
	Status      SanctionCheckStatus
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
	MatchId    string
	ReviewerId UserId
	Status     string
}

type SanctionCheckRefineRequest struct {
	DecisionId string
}

type SanctionCheckMatchComment struct {
	Id          string
	MatchId     string
	CommenterId UserId
	Comment     string
	CreatedAt   time.Time
}
