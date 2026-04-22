package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type ScoringRulesetsUsecase struct {
	mock.Mock
}

func (m *ScoringRulesetsUsecase) CommittedRulesetExists(ctx context.Context, orgId uuid.UUID, recordType string) (bool, error) {
	args := m.Called(ctx, orgId, recordType)

	return args.Bool(0), args.Error(1)
}
