package models

type EmbeddingType int

// Look at GlobalDashboard to fill all places where it is used (no pattern matching enforced by the compilator in Go)
const (
	GlobalDashboard EmbeddingType = iota
)

func (embeddingType EmbeddingType) String() string {
	switch embeddingType {
	case GlobalDashboard:
		return "global_dashboard"
	default:
		panic("unknown embedding type")
	}
}

// Metabase resource type used for embedding (e.g. dashboard, question...)
func (embeddingType EmbeddingType) ResourceType() string {
	switch embeddingType {
	case GlobalDashboard:
		return "dashboard"
	default:
		panic("unknown embedding type")
	}
}

type Analytics struct {
	OrganizationId     string
	EmbeddingType      EmbeddingType
	SignedEmbeddingURL string
}

type AnalyticsCustomClaims interface {
	GetEmbeddingType() EmbeddingType
	GetParams() map[string]interface{}
}

type GlobalDashboardAnalytics struct {
	OrganizationId string
}

func (analytics GlobalDashboardAnalytics) GetEmbeddingType() EmbeddingType {
	return GlobalDashboard
}

func (analytics GlobalDashboardAnalytics) GetParams() map[string]interface{} {
	return map[string]interface{}{
		"organization_id": []string{analytics.OrganizationId},
	}
}
