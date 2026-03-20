package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type ScoringScoreUsecase struct {
	mock.Mock
}

func (m *ScoringScoreUsecase) EnqueueComputationForDecisions(ctx context.Context, orgId uuid.UUID, decisions []models.Decision) error {
	args := m.Called(ctx, orgId, decisions)

	return args.Error(0)
}

func (m *ScoringScoreUsecase) EnqueueComputationForIngestion(ctx context.Context, orgId uuid.UUID, recordType string, records models.IngestionResults) error {
	args := m.Called(ctx, orgId, recordType, records)

	return args.Error(0)
}
