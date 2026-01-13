package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/mock"
)

type OpenSanctionsRepository struct {
	mock.Mock
}

func (m *OpenSanctionsRepository) IsSelfHosted(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

func (m *OpenSanctionsRepository) GetRawCatalog(ctx context.Context) (models.OpenSanctionsRawCatalog, error) {
	args := m.Called(ctx)
	return args.Get(0).(models.OpenSanctionsRawCatalog), args.Error(1)
}

func (m *OpenSanctionsRepository) GetCatalog(ctx context.Context) (models.OpenSanctionsCatalog, error) {
	args := m.Called(ctx)
	return args.Get(0).(models.OpenSanctionsCatalog), args.Error(1)
}

func (m *OpenSanctionsRepository) GetLatestLocalDataset(ctx context.Context) (models.OpenSanctionsDatasetFreshness, error) {
	args := m.Called(ctx)
	return args.Get(0).(models.OpenSanctionsDatasetFreshness), args.Error(1)
}

func (m *OpenSanctionsRepository) Search(ctx context.Context, query models.OpenSanctionsQuery) (models.ScreeningRawSearchResponseWithMatches, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(models.ScreeningRawSearchResponseWithMatches), args.Error(1)
}

func (m *OpenSanctionsRepository) EnrichMatch(ctx context.Context, match models.ScreeningMatch) ([]byte, error) {
	args := m.Called(ctx, match)
	return args.Get(0).([]byte), args.Error(1)
}
