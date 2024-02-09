package dto

import "github.com/checkmarble/marble-backend/models"

type Analytics struct {
	EmbeddingId        string `json:"embedding_id"`
	SignedEmbeddingURL string `json:"signed_embedding_url"`
}

func AdaptAnalyticsDto(analytics models.Analytics) Analytics {
	return Analytics{
		EmbeddingId:        analytics.EmbeddingId.String(),
		SignedEmbeddingURL: analytics.SignedEmbeddingURL,
	}
}
