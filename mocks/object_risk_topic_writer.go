package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ObjectRiskTopicWriter struct {
	mock.Mock
}

func (m *ObjectRiskTopicWriter) AppendObjectRiskTopics(
	ctx context.Context,
	tx repositories.Transaction,
	input models.ObjectRiskTopicUpsert,
) error {
	args := m.Called(ctx, tx, input)
	return args.Error(0)
}
