package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

type Metabase interface {
	GenerateSignedEmbeddingURL(analyticsCustomClaims models.AnalyticsCustomClaims) (string, error)
}

type MarbleAnalyticsRepository struct {
	metabase Metabase
}

func (repo *MarbleAnalyticsRepository) ListAnalytics(ctx context.Context, organizationId string) ([]models.Analytics, error) {
	var analytics []models.Analytics

	// Add general dashboard
	generalDashboardUrl, err := repo.metabase.GenerateSignedEmbeddingURL(models.AnalyticsCustomClaims{
		Resource: map[string]interface{}{
			"dashboard": 8,
		},
		Params: map[string]interface{}{
			"org_id": []string{organizationId},
		},
	})
	if err != nil {
		return nil, err
	}
	analytics = append(analytics, models.Analytics{
		OrganizationId:     organizationId,
		EmbeddingId:        models.GlobalDashboard,
		SignedEmbeddingURL: generalDashboardUrl,
	})

	return analytics, nil
}
