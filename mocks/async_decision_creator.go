package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type AsyncDecisionCreator struct {
	mock.Mock
}

func (m *AsyncDecisionCreator) CreateAllDecisions(
	ctx context.Context,
	input models.CreateAllDecisionsInput,
	params models.CreateDecisionParams,
	optTx ...repositories.Transaction,
) ([]models.DecisionWithRuleExecutions, int, []string, error) {
	args := m.Called(ctx, input, params, optTx)
	return args.Get(0).([]models.DecisionWithRuleExecutions), args.Int(1),
		args.Get(2).([]string), args.Error(3)
}
