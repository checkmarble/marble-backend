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
	globalDashboardAnalytics, err := repo.getAnalytics(ctx, organizationId, models.GlobalDashboardAnalytics{
		OrganizationId: organizationId,
	})
	if err != nil {
		return nil, err
	}
	analytics = append(analytics, globalDashboardAnalytics)

	return analytics, nil
}

func (repo *MarbleAnalyticsRepository) getAnalytics(ctx context.Context, organizationId string, analyticsCustomClaims models.AnalyticsCustomClaims) (models.Analytics, error) {
	generalDashboardUrl, err := repo.metabase.GenerateSignedEmbeddingURL(analyticsCustomClaims)
	if err != nil {
		return models.Analytics{}, err
	}
	return models.Analytics{
		OrganizationId:     organizationId,
		EmbeddingType:      analyticsCustomClaims.GetEmbeddingType(),
		SignedEmbeddingURL: generalDashboardUrl,
	}, nil
}
