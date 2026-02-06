package models

import (
	"encoding/json"

	"github.com/google/uuid"
)

// RiskTopic represents Marble's risk topic categories.
type RiskTopic string

const (
	RiskTopicUnknown      RiskTopic = "unknown"
	RiskTopicSanctions    RiskTopic = "sanctions"
	RiskTopicPEPs         RiskTopic = "peps"
	RiskTopicAdverseMedia RiskTopic = "adverse-media"
	RiskTopicThirdParties RiskTopic = "third-parties"
)

// RiskTopicFrom converts a string to a RiskTopic.
// Returns RiskTopicUnknown if the string doesn't match any known topic.
func RiskTopicFrom(s string) RiskTopic {
	switch RiskTopic(s) {
	case RiskTopicSanctions, RiskTopicPEPs, RiskTopicAdverseMedia, RiskTopicThirdParties:
		return RiskTopic(s)
	default:
		return RiskTopicUnknown
	}
}

// IsValid returns true if the risk topic is a known valid topic (not unknown).
func (rt RiskTopic) IsValid() bool {
	switch rt {
	case RiskTopicSanctions, RiskTopicPEPs, RiskTopicAdverseMedia, RiskTopicThirdParties:
		return true
	}
	return false
}

// OpenSanctionsTagMapping maps OpenSanctions tags/dataset identifiers to Marble RiskTopics.
// This is the single source of truth for converting OpenSanctions taxonomy to Marble's
// risk topic categories.
var OpenSanctionsTagMapping = map[string]RiskTopic{
	// Direct category tags from OpenSanctions
	"regulatory":       RiskTopicAdverseMedia,
	"debarment":        RiskTopicAdverseMedia,
	"special_interest": RiskTopicAdverseMedia,
	"enrichers":        RiskTopicThirdParties,
	"crime":            RiskTopicAdverseMedia,
	"peps":             RiskTopicPEPs,
	"pep":              RiskTopicPEPs,
	"sanctions":        RiskTopicSanctions,
	"sanction":         RiskTopicSanctions,

	// OpenSanctions upstream/list tags
	"list.sanction":         RiskTopicSanctions,
	"list.sanction.counter": RiskTopicSanctions,
	"list.sanction.eu":      RiskTopicSanctions,
	"list.pep":              RiskTopicPEPs,
	"list.regulatory":       RiskTopicAdverseMedia,
	"list.risk":             RiskTopicAdverseMedia,
	"list.wanted":           RiskTopicAdverseMedia,
}

// MapOpenSanctionsTagToRiskTopic converts an OpenSanctions tag to a Marble RiskTopic.
// Returns RiskTopicUnknown if no mapping exists.
func MapOpenSanctionsTagToRiskTopic(tag string) RiskTopic {
	if topic, ok := OpenSanctionsTagMapping[tag]; ok {
		return topic
	}
	return RiskTopicUnknown
}

// MapOpenSanctionsTagsToRiskTopics converts multiple OpenSanctions tags to unique Marble RiskTopics.
// Unknown tags are silently ignored.
func MapOpenSanctionsTagsToRiskTopics(tags []string) []RiskTopic {
	topicSet := make(map[RiskTopic]struct{})
	for _, tag := range tags {
		if topic := MapOpenSanctionsTagToRiskTopic(tag); topic != RiskTopicUnknown {
			topicSet[topic] = struct{}{}
		}
	}

	topics := make([]RiskTopic, 0, len(topicSet))
	for topic := range topicSet {
		topics = append(topics, topic)
	}
	return topics
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
