package mocks

import (
	"context"
	"net"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type OrganizationRepository struct {
	mock.Mock
}

func (m *OrganizationRepository) GetOrganizationById(
	ctx context.Context,
	exec repositories.Executor,
	organizationId uuid.UUID,
) (models.Organization, error) {
	args := m.Called(ctx, exec, organizationId)
	return args.Get(0).(models.Organization), args.Error(1)
}

func (m *OrganizationRepository) GetOrganizationAllowedNetworks(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID,
) ([]net.IPNet, error) {
	args := m.Called(ctx, exec, orgId)

	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]net.IPNet), args.Error(1)
}

func (m *OrganizationRepository) AllOrganizations(ctx context.Context,
	exec repositories.Executor,
) ([]models.Organization, error) {
	args := m.Called(ctx, exec)

	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]models.Organization), args.Error(1)
}

func (m *OrganizationRepository) CreateOrganization(ctx context.Context,
	exec repositories.Executor, newOrganizationId uuid.UUID, input models.CreateOrganizationInput,
) error {
	args := m.Called(ctx, exec, newOrganizationId, input)
	return args.Error(0)
}

func (m *OrganizationRepository) UpdateOrganization(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID, updateOrganization models.UpdateOrganizationInput,
) error {
	args := m.Called(ctx, exec, orgId, updateOrganization)
	return args.Error(0)
}

func (m *OrganizationRepository) DeleteOrganization(ctx context.Context,
	exec repositories.Executor, organizationId uuid.UUID,
) error {
	args := m.Called(ctx, exec, organizationId)
	return args.Error(0)
}

func (m *OrganizationRepository) DeleteOrganizationDecisionRulesAsync(ctx context.Context,
	exec repositories.Executor, organizationId uuid.UUID,
) {
	m.Called(ctx, exec, organizationId)
}

func (m *OrganizationRepository) GetOrganizationFeatureAccess(ctx context.Context,
	exec repositories.Executor, organizationId uuid.UUID,
) (models.DbStoredOrganizationFeatureAccess, error) {
	args := m.Called(ctx, exec, organizationId)
	return args.Get(0).(models.DbStoredOrganizationFeatureAccess), args.Error(1)
}

func (m *OrganizationRepository) UpdateOrganizationFeatureAccess(ctx context.Context,
	exec repositories.Executor, updateFeatureAccess models.UpdateOrganizationFeatureAccessInput,
) error {
	args := m.Called(ctx, exec, updateFeatureAccess)
	return args.Error(0)
}

func (m *OrganizationRepository) HasOrganizations(ctx context.Context,
	exec repositories.Executor,
) (bool, error) {
	args := m.Called(ctx, exec)
	return args.Bool(0), args.Error(1)
}

func (m *OrganizationRepository) UpdateOrganizationAllowedNetworks(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID, subnets []net.IPNet,
) ([]net.IPNet, error) {
	args := m.Called(ctx, exec, orgId, subnets)

	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]net.IPNet), args.Error(1)
}
