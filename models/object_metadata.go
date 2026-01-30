package models

import (
	"encoding/json"
	"time"

	"github.com/cockroachdb/errors"
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
	if len(data) == 0 {
		return nil, nil
	}

	switch metadataType {
	case MetadataTypeRiskTopics:
		return ParseRiskTopicsMetadata(data)
	default:
		return nil, errors.New("cannot parse metadata content: unhandled metadata type")
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

type ObjectMetadataUpsert struct {
	OrgId        uuid.UUID
	ObjectType   string
	ObjectId     string
	MetadataType MetadataType
	Metadata     MetadataContent
}

type ObjectMetadataFilter struct {
	OrgId        uuid.UUID
	MetadataType MetadataType
	ObjectType   string
	ObjectId     string
}

// =============================================================================
// Risk Topics specific types (convenience wrappers)
// =============================================================================

type RiskTopicsMetadataJSON struct {
	Topics        []string        `json:"topics"`
	SourceType    string          `json:"source_type"`
	SourceDetails json.RawMessage `json:"source_details"`
}

// RiskTopicsMetadata implements MetadataContent for risk_topics metadata type
type RiskTopicsMetadata struct {
	Topics        []RiskTopic
	SourceType    RiskTopicSourceType
	SourceDetails SourceDetails
}

func (r RiskTopicsMetadata) MetadataContentType() MetadataType {
	return MetadataTypeRiskTopics
}

func (r RiskTopicsMetadata) ToJSON() (json.RawMessage, error) {
	topics := make([]string, 0, len(r.Topics))
	for _, t := range r.Topics {
		topics = append(topics, string(t))
	}

	output := RiskTopicsMetadataJSON{
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
func ParseRiskTopicsMetadata(data json.RawMessage) (RiskTopicsMetadata, error) {
	if data == nil {
		return RiskTopicsMetadata{}, nil
	}

	var raw RiskTopicsMetadataJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return RiskTopicsMetadata{}, err
	}

	topics := make([]RiskTopic, 0, len(raw.Topics))
	for _, t := range raw.Topics {
		topics = append(topics, RiskTopicFrom(t))
	}

	sourceType := RiskTopicSourceTypeFrom(raw.SourceType)
	sourceDetails, err := ParseSourceDetails(sourceType, raw.SourceDetails)
	if err != nil {
		return RiskTopicsMetadata{}, err
	}

	return RiskTopicsMetadata{
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

// ObjectRiskTopicUpsert contains all data needed for upsert operation
type ObjectRiskTopicUpsert struct {
	OrgId         uuid.UUID
	ObjectType    string
	ObjectId      string
	Topics        []RiskTopic
	SourceType    RiskTopicSourceType
	SourceDetails SourceDetails
}

func NewObjectRiskTopicFromManualUpsert(
	orgId uuid.UUID,
	objectType string,
	objectId string,
	topics []RiskTopic,
	reason string,
	proofUrl string,
) ObjectRiskTopicUpsert {
	return ObjectRiskTopicUpsert{
		OrgId:      orgId,
		ObjectType: objectType,
		ObjectId:   objectId,
		Topics:     topics,
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
) ObjectRiskTopicUpsert {
	return ObjectRiskTopicUpsert{
		OrgId:      orgId,
		ObjectType: objectType,
		ObjectId:   objectId,
		Topics:     topics,
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

type ObjectRiskTopicsMetadataFilter struct {
	OrgId      uuid.UUID
	ObjectType string
	ObjectIds  []string
	Topics     []RiskTopic
}
