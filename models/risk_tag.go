package models

import (
	"encoding/json"
	"slices"

	"github.com/google/uuid"
)

// RiskTag represents Marble's risk tag categories.
type RiskTag string

const (
	RiskTagUnknown      RiskTag = "unknown"
	RiskTagSanctions    RiskTag = "sanctions"
	RiskTagPEPs         RiskTag = "peps"
	RiskTagAdverseMedia RiskTag = "adverse-media"
	RiskTagThirdParties RiskTag = "third-parties"
)

var ValidRiskTags = []RiskTag{
	RiskTagSanctions,
	RiskTagPEPs,
	RiskTagAdverseMedia,
	RiskTagThirdParties,
}

// RiskTagFrom converts a string to a RiskTag.
// Returns RiskTagUnknown if the string doesn't match any known tag.
func RiskTagFrom(s string) RiskTag {
	if slices.Contains(ValidRiskTags, RiskTag(s)) {
		return RiskTag(s)
	}
	return RiskTagUnknown
}

// OpenSanctionsTopicMapping maps OpenSanctions tags/dataset identifiers to Marble RiskTags.
// This is the single source of truth for converting OpenSanctions taxonomy to Marble's
// risk tag categories.
var OpenSanctionsTopicMapping = map[string]RiskTag{
	// Direct category tags from OpenSanctions
	"regulatory":       RiskTagAdverseMedia,
	"debarment":        RiskTagAdverseMedia,
	"special_interest": RiskTagAdverseMedia,
	"enrichers":        RiskTagThirdParties,
	"crime":            RiskTagAdverseMedia,
	"peps":             RiskTagPEPs,
	"pep":              RiskTagPEPs,
	"sanctions":        RiskTagSanctions,
	"sanction":         RiskTagSanctions,

	// OpenSanctions upstream/list tags
	"list.sanction":         RiskTagSanctions,
	"list.sanction.counter": RiskTagSanctions,
	"list.sanction.eu":      RiskTagSanctions,
	"list.pep":              RiskTagPEPs,
	"list.regulatory":       RiskTagAdverseMedia,
	"list.risk":             RiskTagAdverseMedia,
	"list.wanted":           RiskTagAdverseMedia,
}

// MapOpenSanctionsTopicToRiskTag converts an OpenSanctions tag to a Marble RiskTag.
// Returns RiskTagUnknown if no mapping exists.
func MapOpenSanctionsTopicToRiskTag(tag string) RiskTag {
	if riskTag, ok := OpenSanctionsTopicMapping[tag]; ok {
		return riskTag
	}
	return RiskTagUnknown
}

// MapOpenSanctionsTopicToRiskTags converts multiple OpenSanctions topics to unique Marble RiskTags.
// Unknown tags are silently ignored.
func MapOpenSanctionsTopicToRiskTags(tags []string) []RiskTag {
	tagSet := make(map[RiskTag]struct{})
	for _, tag := range tags {
		if riskTag := MapOpenSanctionsTopicToRiskTag(tag); riskTag != RiskTagUnknown {
			tagSet[riskTag] = struct{}{}
		}
	}

	riskTags := make([]RiskTag, 0, len(tagSet))
	for riskTag := range tagSet {
		riskTags = append(riskTags, riskTag)
	}
	return riskTags
}

// ObjectRiskTagCreate contains all data needed to create risk tag annotations.
// Each tag becomes its own entity_annotation row.
type ObjectRiskTagCreate struct {
	OrgId                 uuid.UUID
	ObjectType            string
	ObjectId              string
	Tags                  []RiskTag
	Reason                string
	Url                   string
	ContinuousScreeningId string
	OpenSanctionsEntityId string
	AnnotatedBy           *UserId
}

func NewObjectRiskTagFromContinuousScreeningReview(
	orgId uuid.UUID,
	objectType string,
	objectId string,
	tags []RiskTag,
	sourceContinuousScreeningId uuid.UUID,
	sourceOpenSanctionsEntityId string,
) ObjectRiskTagCreate {
	return ObjectRiskTagCreate{
		OrgId:                 orgId,
		ObjectType:            objectType,
		ObjectId:              objectId,
		Tags:                  tags,
		ContinuousScreeningId: sourceContinuousScreeningId.String(),
		OpenSanctionsEntityId: sourceOpenSanctionsEntityId,
	}
}

// ExtractRiskTagsFromEntityPayload parses the entity payload and converts
// Properties["topics"] to RiskTag values using the shared OpenSanctionsTagMapping.
func ExtractRiskTagsFromEntityPayload(payload []byte) ([]RiskTag, error) {
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

	return MapOpenSanctionsTopicToRiskTags(entityTopics), nil
}
