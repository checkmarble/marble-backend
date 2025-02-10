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
	Id                  string
	DecisionId          string
	Status              SanctionCheckStatus
	Datasets            []string
	SearchInput         json.RawMessage
	OrgConfig           OrganizationOpenSanctionsConfig
	IsManual            bool
	IsArchived          bool
	InitialHasMatches   bool
	RequestedBy         *string
	Partial             bool
	WhitelistedEntities []string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type SanctionCheckWithMatches struct {
	SanctionCheck
	Matches []SanctionCheckMatch
	Count   int
}

type SanctionRawSearchResponseWithMatches struct {
	SearchInput         json.RawMessage
	InitialHasMatches   bool
	WhitelistedEntities []string
	Partial             bool

	Matches []SanctionCheckMatch
	Count   int
}

func (s SanctionRawSearchResponseWithMatches) AdaptSanctionCheckFromSearchResponse(query OpenSanctionsQuery) SanctionCheckWithMatches {
	sanctionCheck := SanctionCheckWithMatches{
		SanctionCheck: SanctionCheck{
			Datasets:            query.Config.Datasets,
			OrgConfig:           query.OrgConfig,
			SearchInput:         s.SearchInput,
			Partial:             s.Partial,
			InitialHasMatches:   s.InitialHasMatches,
			WhitelistedEntities: s.WhitelistedEntities,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		},
		Matches: s.Matches,
		Count:   s.Count,
	}
	sanctionCheck.Status = sanctionCheck.InitialStatusFromMatches()
	return sanctionCheck
}

func (s SanctionCheckWithMatches) InitialStatusFromMatches() SanctionCheckStatus {
	if len(s.Matches) == 0 {
		return SanctionStatusNoHit
	}

	if s.Partial {
		return SanctionStatusTooManyHits
	}

	return SanctionStatusInReview
}

type SanctionCheckMatch struct {
	Id              string
	IsMatch         bool
	SanctionCheckId string
	EntityId        string
	Status          SanctionCheckMatchStatus
	QueryIds        []string
	Payload         []byte
	ReviewedBy      *string
	Comments        []SanctionCheckMatchComment
}

type SanctionCheckMatchUpdate struct {
	MatchId    string
	ReviewerId UserId
	Status     SanctionCheckMatchStatus
	Comment    *SanctionCheckMatchComment
}

type SanctionCheckRefineRequest struct {
	DecisionId string
	Type       string
	Query      OpenSanctionCheckFilter
}

type SanctionCheckMatchComment struct {
	Id          string
	MatchId     string
	CommenterId UserId
	Comment     string
	CreatedAt   time.Time
}

type SanctionCheckFile struct {
	Id              string
	SanctionCheckId string
	BucketName      string
	FileReference   string
	FileName        string
	CreatedAt       time.Time
}

type SanctionCheckFileInput struct {
	SanctionCheckId string
	BucketName      string
	FileReference   string
	FileName        string
}

type SanctionCheckWhitelist struct {
	Id             string
	OrgId          string
	CounterpartyId string
	EntityId       string
	WhitelistedBy  string
	CreatedAt      time.Time
}
