package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// RiskTopic type and constants are defined in risk_topic.go

type RiskTopicSourceType int

const (
	RiskTopicSourceTypeUnknown RiskTopicSourceType = iota
	RiskTopicSourceTypeContinuousScreeningMatchReview
	RiskTopicSourceTypeManual
)

func RiskTopicSourceTypeFrom(s string) RiskTopicSourceType {
	switch s {
	case "continuous_screening_match_review":
		return RiskTopicSourceTypeContinuousScreeningMatchReview
	case "manual":
		return RiskTopicSourceTypeManual
	}

	return RiskTopicSourceTypeUnknown
}

func (rtst RiskTopicSourceType) String() string {
	switch rtst {
	case RiskTopicSourceTypeContinuousScreeningMatchReview:
		return "continuous_screening_match_review"
	case RiskTopicSourceTypeManual:
		return "manual"
	}

	return "unknown"
}

// SourceDetails is an interface for different source detail types
type SourceDetails interface {
	SourceDetailType() RiskTopicSourceType
	ToJSON() (json.RawMessage, error)
}

// ContinuousScreeningSourceDetails for continuous_screening_match_review source type
type ContinuousScreeningSourceDetails struct {
	ContinuousScreeningId uuid.UUID `json:"continuous_screening_id"`
	OpenSanctionsEntityId string    `json:"opensanctions_entity_id"` //nolint: tagliatelle
}

func (s ContinuousScreeningSourceDetails) SourceDetailType() RiskTopicSourceType {
	return RiskTopicSourceTypeContinuousScreeningMatchReview
}

func (s ContinuousScreeningSourceDetails) ToJSON() (json.RawMessage, error) {
	return json.Marshal(s)
}

// ManualSourceDetails for manual source type
type ManualSourceDetails struct {
	Reason string `json:"reason,omitempty"`
	Url    string `json:"url,omitempty"`
}

func (m ManualSourceDetails) SourceDetailType() RiskTopicSourceType {
	return RiskTopicSourceTypeManual
}

func (m ManualSourceDetails) ToJSON() (json.RawMessage, error) {
	return json.Marshal(m)
}

// ParseSourceDetails parses JSON into the appropriate SourceDetails type
func ParseSourceDetails(sourceType RiskTopicSourceType, data json.RawMessage) (SourceDetails, error) {
	if data == nil {
		return nil, nil
	}

	switch sourceType {
	case RiskTopicSourceTypeContinuousScreeningMatchReview:
		var details ContinuousScreeningSourceDetails
		if err := json.Unmarshal(data, &details); err != nil {
			return nil, err
		}
		return details, nil
	case RiskTopicSourceTypeManual:
		var details ManualSourceDetails
		if err := json.Unmarshal(data, &details); err != nil {
			return nil, err
		}
		return details, nil
	default:
		return nil, nil
	}
}

type ObjectRiskTopic struct {
	Id         uuid.UUID
	OrgId      uuid.UUID
	ObjectType string
	ObjectId   string
	Topics     []RiskTopic
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ObjectRiskTopicCreate struct {
	OrgId      uuid.UUID
	ObjectType string
	ObjectId   string
	Topics     []RiskTopic
}

type ObjectRiskTopicFilter struct {
	OrgId      uuid.UUID
	ObjectType *string
	ObjectIds  []string
	Topics     []RiskTopic
}

type ObjectRiskTopicEvent struct {
	Id                 uuid.UUID
	OrgId              uuid.UUID
	ObjectRiskTopicsId uuid.UUID
	Topics             []RiskTopic
	SourceType         RiskTopicSourceType
	SourceDetails      SourceDetails
	UserId             *uuid.UUID
	ApiKeyId           *uuid.UUID
	CreatedAt          time.Time
}

type ObjectRiskTopicEventCreate struct {
	OrgId              uuid.UUID
	ObjectRiskTopicsId uuid.UUID
	Topics             []RiskTopic
	SourceType         RiskTopicSourceType
	SourceDetails      SourceDetails
	UserId             *uuid.UUID
	ApiKeyId           *uuid.UUID
}

type ObjectRiskTopicWithEventUpsert struct {
	OrgId         uuid.UUID
	ObjectType    string
	ObjectId      string
	Topics        []RiskTopic
	SourceType    RiskTopicSourceType
	SourceDetails SourceDetails
	UserId        uuid.UUID
}

func NewObjectRiskTopicWithEventFromManualUpsert(
	orgId uuid.UUID,
	objectType string,
	objectId string,
	topics []RiskTopic,
	userId uuid.UUID,
	reason string,
	proofUrl string,
) ObjectRiskTopicWithEventUpsert {
	return ObjectRiskTopicWithEventUpsert{
		OrgId:      orgId,
		ObjectType: objectType,
		ObjectId:   objectId,
		Topics:     topics,
		UserId:     userId,
		SourceType: RiskTopicSourceTypeManual,
		SourceDetails: ManualSourceDetails{
			Reason: reason,
			Url:    proofUrl,
		},
	}
}

func NewObjectRiskTopicWithEventFromContinuousScreeningReviewUpsert(
	orgId uuid.UUID,
	objectType string,
	objectId string,
	topics []RiskTopic,
	sourceContinuousScreeningId uuid.UUID,
	sourceOpenSanctionsEntityId string,
	userId uuid.UUID,
) ObjectRiskTopicWithEventUpsert {
	return ObjectRiskTopicWithEventUpsert{
		OrgId:      orgId,
		ObjectType: objectType,
		ObjectId:   objectId,
		Topics:     topics,
		UserId:     userId,
		SourceType: RiskTopicSourceTypeContinuousScreeningMatchReview,
		SourceDetails: ContinuousScreeningSourceDetails{
			ContinuousScreeningId: sourceContinuousScreeningId,
			OpenSanctionsEntityId: sourceOpenSanctionsEntityId,
		},
	}
}

// ExtractRiskTopicsFromEntityPayload parses the entity payload and converts
// Properties["topics"] to RiskTopic values using the shared OpenSanctionsTagMapping.
func ExtractRiskTopicsFromEntityPayload(payload []byte) ([]RiskTopic, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	var entity struct {
		Properties struct {
			Topics []string `json:"topics"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(payload, &entity); err != nil {
		return nil, err
	}

	entityTopics := entity.Properties.Topics
	if len(entityTopics) == 0 {
		return nil, nil
	}

	return MapOpenSanctionsTagsToRiskTopics(entityTopics), nil
}
