package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/mock"
)

type OpenSanctionsProvider struct {
	mock.Mock
}

func (m *OpenSanctionsProvider) IsConfigured(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *OpenSanctionsProvider) IsSelfHosted(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

func (m *OpenSanctionsProvider) EnrichMatch(ctx context.Context, match models.ScreeningMatch) ([]byte, error) {
	args := m.Called(ctx, match)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *OpenSanctionsProvider) GetAlgorithms(ctx context.Context) (models.OpenSanctionAlgorithms, error) {
	args := m.Called(ctx)
	return args.Get(0).(models.OpenSanctionAlgorithms), args.Error(1)
}

func (m *OpenSanctionsProvider) Search(
	ctx context.Context,
	query models.OpenSanctionsQuery,
) (models.ScreeningRawSearchResponseWithMatches, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return models.ScreeningRawSearchResponseWithMatches{}, args.Error(1)
	}
	return args.Get(0).(models.ScreeningRawSearchResponseWithMatches), args.Error(1)
}
