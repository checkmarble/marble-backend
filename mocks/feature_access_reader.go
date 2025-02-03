package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/mock"
)

type FeatureAccessReader struct {
	mock.Mock
}

func (r *FeatureAccessReader) GetOrganizationFeatureAccess(ctx context.Context,
	organizationId string,
) (models.OrganizationFeatureAccess, error) {
	args := r.Called(ctx, organizationId)
	return args.Get(0).(models.OrganizationFeatureAccess), args.Error(1)
}
