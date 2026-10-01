package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type CaseEntityDecisionsRepository struct {
	repositories.DecisionRepository
	mock.Mock
}

func (m *CaseEntityDecisionsRepository) DecisionsByCaseId(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, caseId string) ([]models.Decision, error) {
	args := m.Called(ctx, exec, orgId, caseId)
	return args.Get(0).([]models.Decision), args.Error(1)
}
