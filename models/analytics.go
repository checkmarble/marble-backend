package models

type EmbeddingType int

const (
	GlobalDashboard EmbeddingType = iota
)

func (id EmbeddingType) String() string {
	switch id {
	case GlobalDashboard:
		return "global_dashboard"
	default:
		return "unknown_embedding_type"
	}
}

type Analytics struct {
	OrganizationId     string
	EmbeddingType      EmbeddingType
	SignedEmbeddingURL string
}

type AnalyticsCustomClaims struct {
	Resource map[string]interface{}
	Params   map[string]interface{}
}
