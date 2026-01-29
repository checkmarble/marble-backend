package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ObjectRiskTopicRepository struct {
	mock.Mock
}

func (m *ObjectRiskTopicRepository) GetDataModel(
	ctx context.Context,
	exec repositories.Executor,
	organizationID uuid.UUID,
	fetchEnumValues bool,
	useCache bool,
) (models.DataModel, error) {
	args := m.Called(ctx, exec, organizationID, fetchEnumValues, useCache)
	return args.Get(0).(models.DataModel), args.Error(1)
}

func (m *ObjectRiskTopicRepository) GetObjectRiskTopicById(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
) (models.ObjectRiskTopic, error) {
	args := m.Called(ctx, exec, id)
	return args.Get(0).(models.ObjectRiskTopic), args.Error(1)
}

func (m *ObjectRiskTopicRepository) GetObjectRiskTopicByObjectId(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	objectType string,
	objectId string,
) (models.ObjectRiskTopic, error) {
	args := m.Called(ctx, exec, orgId, objectType, objectId)
	return args.Get(0).(models.ObjectRiskTopic), args.Error(1)
}

func (m *ObjectRiskTopicRepository) ListObjectRiskTopics(
	ctx context.Context,
	exec repositories.Executor,
	filter models.ObjectRiskTopicFilter,
	paginationAndSorting models.PaginationAndSorting,
) ([]models.ObjectRiskTopic, error) {
	args := m.Called(ctx, exec, filter, paginationAndSorting)
	return args.Get(0).([]models.ObjectRiskTopic), args.Error(1)
}

func (m *ObjectRiskTopicRepository) UpsertObjectRiskTopic(
	ctx context.Context,
	exec repositories.Executor,
	input models.ObjectRiskTopicCreate,
) (models.ObjectRiskTopic, error) {
	args := m.Called(ctx, exec, input)
	return args.Get(0).(models.ObjectRiskTopic), args.Error(1)
}

func (m *ObjectRiskTopicRepository) InsertObjectRiskTopicEvent(
	ctx context.Context,
	exec repositories.Executor,
	event models.ObjectRiskTopicEventCreate,
) error {
	args := m.Called(ctx, exec, event)
	return args.Error(0)
}

func (m *ObjectRiskTopicRepository) ListObjectRiskTopicEvents(
	ctx context.Context,
	exec repositories.Executor,
	objectRiskTopicsId uuid.UUID,
) ([]models.ObjectRiskTopicEvent, error) {
	args := m.Called(ctx, exec, objectRiskTopicsId)
	return args.Get(0).([]models.ObjectRiskTopicEvent), args.Error(1)
}
