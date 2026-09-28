package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type GrantRepository struct{ mock.Mock }

func (r *GrantRepository) EnsureTenantAdminForOrganization(ctx context.Context, exec repositories.Executor, userID string, organizationID uuid.UUID) error {
	args := r.Called(ctx, exec, userID, organizationID)
	return args.Error(0)
}

func (r *GrantRepository) ListOrganizationsForUser(ctx context.Context, exec repositories.Executor, userID string) ([]models.OrganizationMembership, error) {
	args := r.Called(ctx, exec, userID)
	return args.Get(0).([]models.OrganizationMembership), args.Error(1)
}

func (r *GrantRepository) ListTenantUsersWithDirectOrganizationGrant(ctx context.Context, exec repositories.Executor, tenantID, organizationID uuid.UUID) ([]models.OrganizationUserGrant, error) {
	args := r.Called(ctx, exec, tenantID, organizationID)
	return args.Get(0).([]models.OrganizationUserGrant), args.Error(1)
}

func (r *GrantRepository) ListTenantUsersWithoutOrganizationAccess(ctx context.Context, exec repositories.Executor, tenantID, organizationID uuid.UUID) ([]models.OrganizationUserGrant, error) {
	args := r.Called(ctx, exec, tenantID, organizationID)
	return args.Get(0).([]models.OrganizationUserGrant), args.Error(1)
}

func (r *GrantRepository) ReassignTenantGrants(ctx context.Context, tx repositories.Transaction, targetId uuid.UUID, sourceIds []uuid.UUID) error {
	args := r.Called(ctx, tx, targetId, sourceIds)
	return args.Error(0)
}
