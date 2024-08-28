package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

type AnalyticsRepository interface {
	ListAnalytics(ctx context.Context, organizationId string) ([]models.Analytics, error)
}

type EnforceSecurityAnalytics interface {
	ReadAnalytics(analytics models.Analytics) error
}

type AnalyticsUseCase struct {
	enforceSecurity     EnforceSecurityAnalytics
	analyticsRepository AnalyticsRepository
}

func (usecase *AnalyticsUseCase) ListAnalytics(ctx context.Context, organizationId string) ([]models.Analytics, error) {
	analyticsList, err := usecase.analyticsRepository.ListAnalytics(ctx, organizationId)
	if err != nil {
		return []models.Analytics{}, err
	}
	for _, analytics := range analyticsList {
		if err := usecase.enforceSecurity.ReadAnalytics(analytics); err != nil {
			return []models.Analytics{}, err
		}
	}
	return analyticsList, nil
}
