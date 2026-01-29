package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ObjectRiskTopic struct {
	mock.Mock
}

func (m *ObjectRiskTopic) ListObjectRiskTopics(
	ctx context.Context,
	filter models.ObjectRiskTopicFilter,
	paginationAndSorting models.PaginationAndSorting,
) ([]models.ObjectRiskTopic, error) {
	args := m.Called(ctx, filter, paginationAndSorting)
	return args.Get(0).([]models.ObjectRiskTopic), args.Error(1)
}

func (m *ObjectRiskTopic) GetObjectRiskTopicById(
	ctx context.Context,
	id uuid.UUID,
) (models.ObjectRiskTopic, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.ObjectRiskTopic), args.Error(1)
}

func (m *ObjectRiskTopic) UpsertObjectRiskTopic(
	ctx context.Context,
	input models.ObjectRiskTopicWithEventUpsert,
) (models.ObjectRiskTopic, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(models.ObjectRiskTopic), args.Error(1)
}

func (m *ObjectRiskTopic) AppendObjectRiskTopics(
	ctx context.Context,
	tx repositories.Transaction,
	input models.ObjectRiskTopicWithEventUpsert,
) error {
	args := m.Called(ctx, tx, input)
	return args.Error(0)
}

func (m *ObjectRiskTopic) ListObjectRiskTopicEvents(
	ctx context.Context,
	objectRiskTopicsId uuid.UUID,
) ([]models.ObjectRiskTopicEvent, error) {
	args := m.Called(ctx, objectRiskTopicsId)
	return args.Get(0).([]models.ObjectRiskTopicEvent), args.Error(1)
}
