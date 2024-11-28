package mocks

import (
	"context"

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
