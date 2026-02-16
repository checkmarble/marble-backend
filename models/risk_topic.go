package models

import (
	"encoding/json"
	"slices"

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

var ValidRiskTopics = []RiskTopic{
	RiskTopicSanctions,
	RiskTopicPEPs,
	RiskTopicAdverseMedia,
	RiskTopicThirdParties,
}

// RiskTopicFrom converts a string to a RiskTopic.
// Returns RiskTopicUnknown if the string doesn't match any known topic.
func RiskTopicFrom(s string) RiskTopic {
	if slices.Contains(ValidRiskTopics, RiskTopic(s)) {
		return RiskTopic(s)
	}
	return RiskTopicUnknown
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

// ObjectRiskTopicCreate contains all data needed to create risk topic annotations.
// Each topic becomes its own entity_annotation row.
type ObjectRiskTopicCreate struct {
	OrgId                 uuid.UUID
	ObjectType            string
	ObjectId              string
	Topics                []RiskTopic
	Reason                string
	Url                   string
	ContinuousScreeningId string
	OpenSanctionsEntityId string
	AnnotatedBy           *UserId
}

func NewObjectRiskTopicFromContinuousScreeningReview(
	orgId uuid.UUID,
	objectType string,
	objectId string,
	topics []RiskTopic,
	sourceContinuousScreeningId uuid.UUID,
	sourceOpenSanctionsEntityId string,
) ObjectRiskTopicCreate {
	return ObjectRiskTopicCreate{
		OrgId:                 orgId,
		ObjectType:            objectType,
		ObjectId:              objectId,
		Topics:                topics,
		ContinuousScreeningId: sourceContinuousScreeningId.String(),
		OpenSanctionsEntityId: sourceOpenSanctionsEntityId,
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
