package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type CaseRepository struct {
	mock.Mock
}

func (r *CaseRepository) ListCases(tx repositories.Transaction, organizationId string) ([]models.Case, error) {
	args := r.Called(tx, organizationId)
	return args.Get(0).([]models.Case), args.Error(1)
}
