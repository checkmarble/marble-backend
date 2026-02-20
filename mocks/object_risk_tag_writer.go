package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ObjectRiskTagWriter struct {
	mock.Mock
}

func (m *ObjectRiskTagWriter) AttachObjectRiskTags(
	ctx context.Context,
	tx repositories.Transaction,
	input models.ObjectRiskTagCreate,
) error {
	args := m.Called(ctx, tx, input)
	return args.Error(0)
}
