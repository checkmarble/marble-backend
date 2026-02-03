package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ObjectMetadata struct {
	mock.Mock
}

func (m *ObjectMetadata) UpsertObjectRiskTopic(
	ctx context.Context,
	input models.ObjectRiskTopicUpsert,
) (models.ObjectMetadata, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(models.ObjectMetadata), args.Error(1)
}

func (m *ObjectMetadata) AppendObjectRiskTopics(
	ctx context.Context,
	tx repositories.Transaction,
	input models.ObjectRiskTopicUpsert,
) error {
	args := m.Called(ctx, tx, input)
	return args.Error(0)
}
