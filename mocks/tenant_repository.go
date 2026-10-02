package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type TenantRepository struct{ mock.Mock }

func (r *TenantRepository) ListActive(ctx context.Context, exec repositories.Executor) ([]models.Tenant, error) {
	args := r.Called(ctx, exec)
	return args.Get(0).([]models.Tenant), args.Error(1)
}

func (r *TenantRepository) LockAndValidateMerge(ctx context.Context, exec repositories.Executor, targetId uuid.UUID, sourceIds []uuid.UUID) error {
	args := r.Called(ctx, exec, targetId, sourceIds)
	return args.Error(0)
}

func (r *TenantRepository) LockAndValidateUpdate(ctx context.Context, exec repositories.Executor, tenantId uuid.UUID) error {
	args := r.Called(ctx, exec, tenantId)
	return args.Error(0)
}

func (r *TenantRepository) SoftDelete(ctx context.Context, exec repositories.Executor, tenantIds []uuid.UUID) error {
	args := r.Called(ctx, exec, tenantIds)
	return args.Error(0)
}

func (r *TenantRepository) Rename(ctx context.Context, exec repositories.Executor, tenantId uuid.UUID, name string) error {
	args := r.Called(ctx, exec, tenantId, name)
	return args.Error(0)
}
