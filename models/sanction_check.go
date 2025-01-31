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
	SanctionStatusTooManyHits
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
	case "too_many_hits":
		return SanctionStatusTooManyHits
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
	case SanctionStatusTooManyHits:
		return "too_many_hits"
	}

	return "unknown"
}

func (scs SanctionCheckStatus) IsReviewable() bool {
	return scs == SanctionStatusInReview
}

type SanctionCheckMatchStatus int

const (
	SanctionMatchStatusConfirmedHit SanctionCheckMatchStatus = iota
	SanctionMatchStatusNoHit
	SanctionMatchStatusPending
	SanctionMatchStatusSkipped
	SanctionMatchStatusUnknown
)

func SanctionCheckMatchStatusFrom(s string) SanctionCheckMatchStatus {
	switch s {
	case "confirmed_hit":
		return SanctionMatchStatusConfirmedHit
	case "no_hit":
		return SanctionMatchStatusNoHit
	case "pending":
		return SanctionMatchStatusPending
	case "skipped":
		return SanctionMatchStatusSkipped
	}

	return SanctionMatchStatusUnknown
}

func (scs SanctionCheckMatchStatus) String() string {
	switch scs {
	case SanctionMatchStatusConfirmedHit:
		return "confirmed_hit"
	case SanctionMatchStatusNoHit:
		return "no_hit"
	case SanctionMatchStatusPending:
		return "pending"
	case SanctionMatchStatusSkipped:
		return "skipped"
	}

	return "unknown"
}

type SanctionCheck struct {
	Id          string
	DecisionId  string
	Status      SanctionCheckStatus
	Datasets    []string
	Query       json.RawMessage
	OrgConfig   OrganizationOpenSanctionsConfig
	IsManual    bool
	IsArchived  bool
	RequestedBy *string
	Partial     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SanctionCheckWithMatches struct {
	SanctionCheck
	Matches []SanctionCheckMatch
	Count   int
}

type SanctionCheckMatch struct {
	Id              string
	SanctionCheckId string
	EntityId        string
	Status          SanctionCheckMatchStatus
	QueryIds        []string
	Payload         []byte
	ReviewedBy      *string
	CommentCount    int
}

type SanctionCheckMatchUpdate struct {
	MatchId    string
	ReviewerId UserId
	Status     SanctionCheckMatchStatus
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
