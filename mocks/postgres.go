package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
)

type Database struct {
	mock.Mock
}

func (m *Database) UserByFirebaseUid(ctx context.Context, firebaseUID string) (models.User, error) {
	args := m.Called(ctx, firebaseUID)
	return args.Get(0).(models.User), args.Error(1)
}

func (m *Database) UserByEmail(ctx context.Context, email string) (models.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(models.User), args.Error(1)
}

func (m *Database) UpdateUserFirebaseUID(ctx context.Context, userID models.UserId, firebaseUID string) error {
	args := m.Called(ctx, userID, firebaseUID)
	return args.Error(0)
}

func (m *Database) GetOrganizationByID(ctx context.Context, organizationID string) (models.Organization, error) {
	args := m.Called(ctx, organizationID)
	return args.Get(0).(models.Organization), args.Error(1)
}

func (m *Database) GetApiKeyByKey(ctx context.Context, key string) (models.ApiKey, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(models.ApiKey), args.Error(1)
}
