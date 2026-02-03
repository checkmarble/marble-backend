package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MetadataType defines the type of metadata stored
type MetadataType int

const (
	MetadataTypeUnknown MetadataType = iota
	MetadataTypeRiskTopics
)

func MetadataTypeFrom(s string) MetadataType {
	switch s {
	case "risk_topics":
		return MetadataTypeRiskTopics
	default:
		return MetadataTypeUnknown
	}
}

func (mt MetadataType) String() string {
	switch mt {
	case MetadataTypeRiskTopics:
		return "risk_topics"
	default:
		return "unknown"
	}
}

// =============================================================================
// MetadataContent interface
// =============================================================================

// MetadataContent is an interface for different metadata content types
type MetadataContent interface {
	MetadataContentType() MetadataType
	ToJSON() (json.RawMessage, error)
}

// ParseMetadataContent parses JSON into the appropriate MetadataContent type
func ParseMetadataContent(metadataType MetadataType, data json.RawMessage) (MetadataContent, error) {
	if data == nil {
		return nil, nil
	}

	switch metadataType {
	case MetadataTypeRiskTopics:
		return ParseRiskTopicsMetadata(data)
	default:
		return nil, nil
	}
}

// ObjectMetadata represents generic metadata attached to an object
type ObjectMetadata struct {
	Id           uuid.UUID
	OrgId        uuid.UUID
	ObjectType   string
	ObjectId     string
	MetadataType MetadataType
	Metadata     MetadataContent
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ObjectMetadataFilter is for filtering metadata queries
type ObjectMetadataFilter struct {
	OrgId         uuid.UUID
	MetadataTypes []MetadataType
	ObjectType    *string
	ObjectIds     []string
}

// =============================================================================
// Risk Topics specific types (convenience wrappers)
// =============================================================================

// RiskTopicsMetadata implements MetadataContent for risk_topics metadata type
type RiskTopicsMetadata struct {
	Topics        []RiskTopic         `json:"topics"`
	SourceType    RiskTopicSourceType `json:"-"`
	SourceDetails SourceDetails       `json:"-"`
}

func (r RiskTopicsMetadata) MetadataContentType() MetadataType {
	return MetadataTypeRiskTopics
}

func (r RiskTopicsMetadata) ToJSON() (json.RawMessage, error) {
	type riskTopicsMetadataJSON struct {
		Topics        []string        `json:"topics"`
		SourceType    string          `json:"source_type"`
		SourceDetails json.RawMessage `json:"source_details,omitempty"`
	}

	topics := make([]string, 0, len(r.Topics))
	for _, t := range r.Topics {
		topics = append(topics, string(t))
	}

	output := riskTopicsMetadataJSON{
		Topics:     topics,
		SourceType: r.SourceType.String(),
	}

	if r.SourceDetails != nil {
		sourceDetailsJSON, err := json.Marshal(r.SourceDetails)
		if err != nil {
			return nil, err
		}
		output.SourceDetails = sourceDetailsJSON
	}

	return json.Marshal(output)
}

// ParseRiskTopicsMetadata parses JSON into RiskTopicsMetadata
func ParseRiskTopicsMetadata(data json.RawMessage) (*RiskTopicsMetadata, error) {
	if data == nil {
		return nil, nil
	}

	var raw struct {
		Topics        []string        `json:"topics"`
		SourceType    string          `json:"source_type"`
		SourceDetails json.RawMessage `json:"source_details"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	topics := make([]RiskTopic, 0, len(raw.Topics))
	for _, t := range raw.Topics {
		topics = append(topics, RiskTopicFrom(t))
	}

	sourceType := RiskTopicSourceTypeFrom(raw.SourceType)
	sourceDetails, err := ParseSourceDetails(sourceType, raw.SourceDetails)
	if err != nil {
		return nil, err
	}

	return &RiskTopicsMetadata{
		Topics:        topics,
		SourceType:    sourceType,
		SourceDetails: sourceDetails,
	}, nil
}

// RiskTopicSourceType enum
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
}

// ContinuousScreeningSourceDetails for continuous_screening_match_review source type
type ContinuousScreeningSourceDetails struct {
	ContinuousScreeningId uuid.UUID `json:"continuous_screening_id"`
	OpenSanctionsEntityId string    `json:"opensanctions_entity_id"` //nolint: tagliatelle
}

func (s ContinuousScreeningSourceDetails) SourceDetailType() RiskTopicSourceType {
	return RiskTopicSourceTypeContinuousScreeningMatchReview
}

// ManualSourceDetails for manual source type
type ManualSourceDetails struct {
	Reason string `json:"reason,omitempty"`
	Url    string `json:"url,omitempty"`
}

func (m ManualSourceDetails) SourceDetailType() RiskTopicSourceType {
	return RiskTopicSourceTypeManual
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

// ObjectRiskTopic is a convenience wrapper for risk_topics metadata
type ObjectRiskTopic struct {
	ObjectMetadata
	Topics        []RiskTopic
	SourceType    RiskTopicSourceType
	SourceDetails SourceDetails
}

// TODO: Will be used in next PRs to filter metadata with risk topics
type ObjectRiskTopicFilter struct {
	OrgId      uuid.UUID
	ObjectType *string
	ObjectIds  []string
	Topics     []RiskTopic
}

// ObjectRiskTopicUpsert contains all data needed for upsert operation
type ObjectRiskTopicUpsert struct {
	OrgId         uuid.UUID
	ObjectType    string
	ObjectId      string
	Topics        []RiskTopic
	SourceType    RiskTopicSourceType
	SourceDetails SourceDetails
	UserId        uuid.UUID
}

func NewObjectRiskTopicFromManualUpsert(
	orgId uuid.UUID,
	objectType string,
	objectId string,
	topics []RiskTopic,
	userId uuid.UUID,
	reason string,
	proofUrl string,
) ObjectRiskTopicUpsert {
	return ObjectRiskTopicUpsert{
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

func NewObjectRiskTopicFromContinuousScreeningReviewUpsert(
	orgId uuid.UUID,
	objectType string,
	objectId string,
	topics []RiskTopic,
	sourceContinuousScreeningId uuid.UUID,
	sourceOpenSanctionsEntityId string,
	userId uuid.UUID,
) ObjectRiskTopicUpsert {
	return ObjectRiskTopicUpsert{
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
