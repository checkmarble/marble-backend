package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type OrganizationSchemaRepository struct {
	mock.Mock
}

func (m *OrganizationSchemaRepository) CreateSchemaIfNotExists(ctx context.Context, exec repositories.Executor) error {
	args := m.Called(ctx, exec)
	return args.Error(0)
}

func (m *OrganizationSchemaRepository) DeleteSchema(ctx context.Context, exec repositories.Executor) error {
	args := m.Called(ctx, exec)
	return args.Error(0)
}

func (m *OrganizationSchemaRepository) CreateTable(ctx context.Context, exec repositories.Executor, tableName string) error {
	args := m.Called(ctx, exec, tableName)
	return args.Error(0)
}

func (m *OrganizationSchemaRepository) CreateField(
	ctx context.Context,
	exec repositories.Executor,
	tableName string,
	field models.CreateFieldInput,
) error {
	args := m.Called(ctx, exec, tableName, field)
	return args.Error(0)
}
