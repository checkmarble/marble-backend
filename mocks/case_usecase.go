package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type CaseEditor struct {
	mock.Mock
}

func (m *CaseEditor) CreateCase(
	ctx context.Context,
	tx repositories.Transaction,
	userId string,
	createCaseAttributes models.CreateCaseAttributes,
	fromEndUser bool,
) (models.Case, error) {
	args := m.Called(ctx, tx, userId, createCaseAttributes, fromEndUser)
	return args.Get(0).(models.Case), args.Error(1)
}
