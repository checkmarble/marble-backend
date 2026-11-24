package models

import (
	"encoding/json"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type ScreeningProvider string

const (
	ScreeningProviderOpenSanctions ScreeningProvider = "opensanctions"
	ScreeningProviderLexisNexis    ScreeningProvider = "lexisnexis"
)

type ScreeningFeature string

const (
	ScreeningFeatureTransactionMonitoring ScreeningFeature = "transaction_monitoring"
	ScreeningFeatureContinuousMonitoring  ScreeningFeature = "continuous_monitoring"
	ScreeningFeatureManualSearch          ScreeningFeature = "manual_search"
)

var (
	ValidScreeningProviderFeature = []ScreeningFeature{ScreeningFeatureTransactionMonitoring, ScreeningFeatureContinuousMonitoring, ScreeningFeatureManualSearch}
	ValidScreeningProviders       = []ScreeningProvider{ScreeningProviderOpenSanctions, ScreeningProviderLexisNexis}
	DefaultScreeningProvider      = ScreeningProviderOpenSanctions
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
	Id                           string
	DecisionId                   string
	OrgId                        uuid.UUID
	ScreeningConfigId            string
	Status                       ScreeningStatus
	Config                       ScreeningConfigRef
	Provider                     ScreeningProvider
	UniqueCounterpartyIdentifier *string
	SearchInput                  json.RawMessage
	InitialQuery                 []OpenSanctionsCheckQuery
	OrgConfig                    OrganizationOpenSanctionsConfig
	IsManual                     bool
	IsArchived                   bool
	InitialHasMatches            bool
	RequestedBy                  *string
	Partial                      bool
	ErrorCodes                   []string
	ErrorDetail                  error `json:"-"` // Only used inline to propagate an error, should not be persisted

	NumberOfMatches int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ScreeningBaseInfo struct {
	Id              string
	DecisionId      string
	OrgId           uuid.UUID
	Status          ScreeningStatus
	RequestedBy     *string
	Partial         bool
	Name            string
	NumberOfMatches int
	CreatedAt       time.Time
}

type ScreeningConfigRef struct {
	Id       string
	StableId string
	Name     string
	Provider ScreeningProvider
	Datasets []string
	Filters  ScreeningConfigFilters
}

type ScreeningWithMatches struct {
	Screening
	Matches            []ScreeningMatch
	EffectiveThreshold int

	Duration                time.Duration
	NameRecognitionDuration time.Duration
}

type ScreeningRawSearchResponseWithMatches struct {
	SearchInput        json.RawMessage
	InitialHasMatches  bool
	Partial            bool
	ErrorCodes         []string
	EffectiveThreshold int

	Matches []ScreeningMatch
	Count   int
}

func (s ScreeningRawSearchResponseWithMatches) AdaptScreeningFromSearchResponse(query OpenSanctionsQuery) ScreeningWithMatches {
	screening := ScreeningWithMatches{
		Screening: Screening{
			ScreeningConfigId: query.Config.Id,
			Provider:          query.Config.Provider,
			Config: ScreeningConfigRef{
				Id:       query.Config.Id,
				StableId: query.Config.StableId,
				Name:     query.Config.Name,
				Provider: query.Config.Provider,
				Datasets: query.Config.Datasets,
				Filters:  query.Config.Filters,
			},
			OrgConfig:         query.OrgConfig,
			SearchInput:       s.SearchInput,
			InitialQuery:      query.InitialQuery,
			Partial:           s.Partial,
			InitialHasMatches: s.InitialHasMatches,
			ErrorCodes:        s.ErrorCodes,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
			NumberOfMatches:   s.Count,
		},
		Matches:            s.Matches,
		EffectiveThreshold: s.EffectiveThreshold,
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
	Id          string
	IsMatch     bool
	ScreeningId string
	EntityId    string
	Referents   []string
	Status      ScreeningMatchStatus
	QueryIds    []string
	Payload     []byte
	Enriched    bool
	ReviewedBy  *string
	Score       float64
	Comments    []ScreeningMatchComment
}

// Score is stored in the Payload and not in a dedicated column when fetching screening from DB.
// We can unmarshal the Payload in the `adapt` method of the repository but the attribute is not used in many places.
func (m ScreeningMatch) GetScoreFromPayload() float64 {
	var parsed struct {
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal(m.Payload, &parsed); err != nil {
		return 0
	}
	return parsed.Score
}

type ScreeningMatchUpdate struct {
	MatchId    string
	ReviewerId *UserId
	Status     ScreeningMatchStatus
	Comment    *ScreeningMatchComment
	Whitelist  bool
}

type ScreeningRefineRequest struct {
	ScreeningId   string
	Type          string
	Query         OpenSanctionsFilter
	LimitOverride *int
}

type ScreeningMatchComment struct {
	Id              string
	MatchId         string
	CommenterId     UserId
	Comment         string
	TentativeReview *ScreeningMatchTentativeReview
	CreatedAt       time.Time
}

func (c ScreeningMatchComment) ToCaseEvent(resourceType CaseEventResourceType, orgId uuid.UUID, caseId string) CreateCaseEventAttributes {
	userId := string(c.CommenterId)

	attrs := CreateCaseEventAttributes{
		OrgId:          orgId,
		CaseId:         caseId,
		UserId:         &userId,
		EventType:      ScreeningMatchCommented,
		ResourceType:   &resourceType,
		ResourceId:     &c.MatchId,
		AdditionalNote: &c.Comment,
	}

	if c.TentativeReview != nil {
		attrs.NewValue = &c.TentativeReview.Status
	}

	return attrs
}

// ScreeningMatchTentativeReview is a non-binding review of a match, attached to a comment. It does
// not change the status of the match. A match has at most one active tentative review.
type ScreeningMatchTentativeReview struct {
	Status  string    `json:"status"`
	InboxId uuid.UUID `json:"inbox_id"`

	// Set when a newer tentative review replaced this one; never persisted in the JSON document.
	SupersededAt *time.Time `json:"-"`
}

func (r ScreeningMatchTentativeReview) Validate() error {
	if status := ScreeningMatchStatusFrom(r.Status); status != ScreeningMatchStatusConfirmedHit &&
		status != ScreeningMatchStatusNoHit {
		return errors.Wrap(BadParameterError, "tentative review status must be one of confirmed_hit or no_hit")
	}

	return nil
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
	OrgId          uuid.UUID
	CounterpartyId string
	EntityId       string
	WhitelistedBy  *string
	CreatedAt      time.Time
}
