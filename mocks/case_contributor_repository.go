package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type CaseContributorRepository struct {
	mock.Mock
}

func (r *CaseContributorRepository) GetCaseContributor(exec repositories.Executor, caseId, userId string) (models.CaseContributor, error) {
	args := r.Called(exec, caseId, userId)
	return args.Get(0).(models.CaseContributor), args.Error(1)
}

func (r *CaseContributorRepository) CreateCaseContributor(exec repositories.Executor, caseId, userId string) error {
	args := r.Called(exec, caseId, userId)
	return args.Error(0)
}
