package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
)

type Database struct {
	mock.Mock
}

func (m *Database) UserByEmail(ctx context.Context, email string) (models.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(models.User), args.Error(1)
}

func (m *Database) GetOrganizationByID(ctx context.Context, organizationID string) (models.Organization, error) {
	args := m.Called(ctx, organizationID)
	return args.Get(0).(models.Organization), args.Error(1)
}

func (m *Database) GetApiKeyByHash(ctx context.Context, hash []byte) (models.ApiKey, error) {
	args := m.Called(ctx, hash)
	return args.Get(0).(models.ApiKey), args.Error(1)
}
