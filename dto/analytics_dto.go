package dto

import "github.com/checkmarble/marble-backend/models"

type Analytics struct {
	EmbeddingType      string `json:"embedding_type"`
	SignedEmbeddingURL string `json:"signed_embedding_url"`
}

func AdaptAnalyticsDto(analytics models.Analytics) Analytics {
	return Analytics{
		EmbeddingType:      analytics.EmbeddingType.String(),
		SignedEmbeddingURL: analytics.SignedEmbeddingURL,
	}
}
