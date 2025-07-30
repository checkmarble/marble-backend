package mocks

import (
	"context"
	"net"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type OrganizationRepository struct {
	mock.Mock
}

func (m *OrganizationRepository) GetOrganizationById(
	ctx context.Context,
	exec repositories.Executor,
	organizationId string,
) (models.Organization, error) {
	args := m.Called(ctx, exec, organizationId)
	return args.Get(0).(models.Organization), args.Error(1)
}

func (m *OrganizationRepository) GetOrganizationSubnets(ctx context.Context, exec repositories.Executor, orgId string) ([]net.IPNet, error) {
	args := m.Called(ctx, exec, orgId)

	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]net.IPNet), args.Error(1)
}
