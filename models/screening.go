package models

import (
	"encoding/json"
	"time"
)

type ScreeningStatus int

const (
	ScreeningStatusConfirmedHit ScreeningStatus = iota
	ScreeningStatusNoHit
	ScreeningStatusInReview
	ScreeningStatusError
	ScreeningStatusUnknown
)

func ScreeningStatusFrom(s string) ScreeningStatus {
	switch s {
	case "confirmed_hit":
		return ScreeningStatusConfirmedHit
	case "no_hit":
		return ScreeningStatusNoHit
	case "in_review":
		return ScreeningStatusInReview
	case "error":
		return ScreeningStatusError
	}

	return ScreeningStatusUnknown
}

func (scs ScreeningStatus) String() string {
	switch scs {
	case ScreeningStatusConfirmedHit:
		return "confirmed_hit"
	case ScreeningStatusNoHit:
		return "no_hit"
	case ScreeningStatusInReview:
		return "in_review"
	case ScreeningStatusError:
		return "error"
	}

	return "unknown"
}

func (scs ScreeningStatus) IsReviewable() bool {
	return scs == ScreeningStatusInReview
}

func (scs ScreeningStatus) IsRefinable() bool {
	return scs == ScreeningStatusInReview || scs == ScreeningStatusError
}

type ScreeningMatchStatus int

const (
	ScreeningMatchStatusConfirmedHit ScreeningMatchStatus = iota
	ScreeningMatchStatusNoHit
	ScreeningMatchStatusPending
	ScreeningMatchStatusSkipped
	ScreeningMatchStatusUnknown
)

func ScreeningMatchStatusFrom(s string) ScreeningMatchStatus {
	switch s {
	case "confirmed_hit":
		return ScreeningMatchStatusConfirmedHit
	case "no_hit":
		return ScreeningMatchStatusNoHit
	case "pending":
		return ScreeningMatchStatusPending
	case "skipped":
		return ScreeningMatchStatusSkipped
	}

	return ScreeningMatchStatusUnknown
}

func (scs ScreeningMatchStatus) String() string {
	switch scs {
	case ScreeningMatchStatusConfirmedHit:
		return "confirmed_hit"
	case ScreeningMatchStatusNoHit:
		return "no_hit"
	case ScreeningMatchStatusPending:
		return "pending"
	case ScreeningMatchStatusSkipped:
		return "skipped"
	}

	return "unknown"
}

type Screening struct {
	Id                  string
	DecisionId          string
	ScreeningConfigId   string
	Status              ScreeningStatus
	Config              ScreeningConfigRef
	Datasets            []string
	SearchInput         json.RawMessage
	InitialQuery        []OpenSanctionsCheckQuery
	OrgConfig           OrganizationOpenSanctionsConfig
	IsManual            bool
	IsArchived          bool
	InitialHasMatches   bool
	RequestedBy         *string
	Partial             bool
	WhitelistedEntities []string
	ErrorCodes          []string
	ErrorDetail         error
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ScreeningConfigRef struct {
	Id       string
	StableId string
	Name     string
}

type ScreeningWithMatches struct {
	Screening
	Matches            []ScreeningMatch
	Count              int
	EffectiveThreshold int

	Duration                time.Duration
	NameRecognitionDuration time.Duration
}

type ScreeningRawSearchResponseWithMatches struct {
	SearchInput         json.RawMessage
	InitialHasMatches   bool
	WhitelistedEntities []string
	Partial             bool
	ErrorCodes          []string
	EffectiveThreshold  int

	Matches []ScreeningMatch
	Count   int
}

func (s ScreeningRawSearchResponseWithMatches) AdaptScreeningFromSearchResponse(query OpenSanctionsQuery) ScreeningWithMatches {
	screening := ScreeningWithMatches{
		Screening: Screening{
			ScreeningConfigId: query.Config.Id,
			Config: ScreeningConfigRef{
				Id:       query.Config.Id,
				StableId: query.Config.StableId,
				Name:     query.Config.Name,
			},
			Datasets:            query.Config.Datasets,
			OrgConfig:           query.OrgConfig,
			SearchInput:         s.SearchInput,
			InitialQuery:        query.InitialQuery,
			Partial:             s.Partial,
			InitialHasMatches:   s.InitialHasMatches,
			WhitelistedEntities: s.WhitelistedEntities,
			ErrorCodes:          s.ErrorCodes,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		},
		Matches:            s.Matches,
		EffectiveThreshold: s.EffectiveThreshold,
		Count:              s.Count,
	}
	screening.Status = screening.InitialStatusFromMatches()
	return screening
}

func (s ScreeningWithMatches) InitialStatusFromMatches() ScreeningStatus {
	if len(s.Matches) == 0 {
		return ScreeningStatusNoHit
	}

	return ScreeningStatusInReview
}

type ScreeningMatch struct {
	Id                           string
	IsMatch                      bool
	ScreeningId                  string
	EntityId                     string
	Referents                    []string
	Status                       ScreeningMatchStatus
	QueryIds                     []string
	UniqueCounterpartyIdentifier *string
	Payload                      []byte
	Enriched                     bool
	ReviewedBy                   *string
	Comments                     []ScreeningMatchComment
}

type ScreeningMatchUpdate struct {
	MatchId    string
	ReviewerId *UserId
	Status     ScreeningMatchStatus
	Comment    *ScreeningMatchComment
	Whitelist  bool
}

type ScreeningRefineRequest struct {
	ScreeningId string
	Type        string
	Query       OpenSanctionsFilter
}

type ScreeningMatchComment struct {
	Id          string
	MatchId     string
	CommenterId UserId
	Comment     string
	CreatedAt   time.Time
}

type ScreeningFile struct {
	Id            string
	ScreeningId   string
	BucketName    string
	FileReference string
	FileName      string
	CreatedAt     time.Time
}

type ScreeningFileInput struct {
	ScreeningId   string
	BucketName    string
	FileReference string
	FileName      string
}

type ScreeningWhitelist struct {
	Id             string
	OrgId          string
	CounterpartyId string
	EntityId       string
	WhitelistedBy  *string
	CreatedAt      time.Time
}
