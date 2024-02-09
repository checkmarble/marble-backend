package models

type EmbeddingId int

const (
	GlobalDashboard EmbeddingId = iota
)

func (id EmbeddingId) String() string {
	switch id {
	case GlobalDashboard:
		return "global_dashboard"
	default:
		return "unknown_embedding_id"
	}
}

type Analytics struct {
	OrganizationId     string
	EmbeddingId        EmbeddingId
	SignedEmbeddingURL string
}

type AnalyticsCustomClaims struct {
	Resource map[string]interface{}
	Params   map[string]interface{}
}
