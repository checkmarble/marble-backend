package models

// RiskTopic represents Marble's risk topic categories.
type RiskTopic int

const (
	RiskTopicUnknown      RiskTopic = iota
	RiskTopicSanctions    
	RiskTopicPEPs         
	RiskTopicAdverseMedia 
	RiskTopicThirdParties 
)

// ValidRiskTopics returns all valid Marble risk topics (excluding unknown).
var ValidRiskTopics = []RiskTopic{
	RiskTopicSanctions,
	RiskTopicPEPs,
	RiskTopicAdverseMedia,
	RiskTopicThirdParties,
}

// RiskTopicFrom converts a string to a RiskTopic.
// Returns RiskTopicUnknown if the string doesn't match any known topic.
func RiskTopicFrom(s string) RiskTopic {
	switch s {
	case "sanctions":
		return RiskTopicSanctions
	case "peps":
		return RiskTopicPEPs
	case "adverse-media":
		return RiskTopicAdverseMedia
	case "third-parties":
		return RiskTopicThirdParties
	}

	return RiskTopicUnknown
}

func (rt RiskTopic) String() string {
	switch rt {
	case RiskTopicSanctions:
		return "sanctions"
	case RiskTopicPEPs:
		return "peps"
	case RiskTopicAdverseMedia:
		return "adverse-media"
	case RiskTopicThirdParties:
		return "third-parties"
	default:
		return "unknown"
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
