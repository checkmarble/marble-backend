package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/mock"
)

type OpenSanctionsRepository struct {
	mock.Mock
}

func (m *OpenSanctionsRepository) GetRawCatalog(ctx context.Context) (models.OpenSanctionsRawCatalog, error) {
	args := m.Called(ctx)
	return args.Get(0).(models.OpenSanctionsRawCatalog), args.Error(1)
}

func (m *OpenSanctionsRepository) Search(
	ctx context.Context,
	query models.OpenSanctionsQuery,
) (models.ScreeningRawSearchResponseWithMatches, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return models.ScreeningRawSearchResponseWithMatches{}, args.Error(1)
	}
	return args.Get(0).(models.ScreeningRawSearchResponseWithMatches), args.Error(1)
}

func (m *OpenSanctionsRepository) GetAlgorithms(ctx context.Context) (models.OpenSanctionAlgorithms, error) {
	args := m.Called(ctx)
	return args.Get(0).(models.OpenSanctionAlgorithms), args.Error(1)
}

func (m *OpenSanctionsRepository) EnrichMatch(ctx context.Context, match models.ScreeningMatch) ([]byte, error) {
	args := m.Called(ctx, match)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *OpenSanctionsRepository) IsConfigured(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *OpenSanctionsRepository) IsSelfHosted(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}
