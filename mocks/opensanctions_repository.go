package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/mock"
)

type OpenSanctionsRepository struct {
	mock.Mock
}

func (m *OpenSanctionsRepository) GetRawCatalog(ctx context.Context) (models.OpenSanctionsRawCatalog, error) {
	args := m.Called(ctx)
	return args.Get(0).(models.OpenSanctionsRawCatalog), args.Error(1)
}
