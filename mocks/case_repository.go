package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type CaseRepository struct {
	mock.Mock
}

func (r *CaseRepository) ListOrganizationCases(exec repositories.Executor, organizationId string,
	filters models.CaseFilters, pagination models.PaginationAndSorting,
) ([]models.CaseWithRank, error) {
	args := r.Called(exec, organizationId)
	return args.Get(0).([]models.CaseWithRank), args.Error(1)
}

func (r *CaseRepository) GetCaseById(exec repositories.Executor, caseId string) (models.Case, error) {
	args := r.Called(exec, caseId)
	return args.Get(0).(models.Case), args.Error(1)
}

func (r *CaseRepository) CreateCase(exec repositories.Executor,
	createCaseAttributes models.CreateCaseAttributes, newCaseId string,
) error {
	args := r.Called(exec, createCaseAttributes, newCaseId)
	return args.Error(0)
}

func (r *CaseRepository) UpdateCase(exec repositories.Executor, caseId string, updateCaseAttributes models.UpdateCaseAttributes) error {
	args := r.Called(exec, caseId, updateCaseAttributes)
	return args.Error(0)
}

func (r *CaseRepository) CreateCaseTag(exec repositories.Executor, newCaseTagId string,
	createCaseTagAttributes models.CreateCaseTagsAttributes,
) error {
	args := r.Called(exec, newCaseTagId, createCaseTagAttributes)
	return args.Error(0)
}

func (r *CaseRepository) ListCaseTagsByCaseId(exec repositories.Executor, caseId string) ([]models.CaseTag, error) {
	args := r.Called(exec, caseId)
	return args.Get(0).([]models.CaseTag), args.Error(1)
}

func (r *CaseRepository) ListCaseTagsByTagId(exec repositories.Executor, tagId string) ([]models.CaseTag, error) {
	args := r.Called(exec, tagId)
	return args.Get(0).([]models.CaseTag), args.Error(1)
}

func (r *CaseRepository) SoftDeleteCaseTag(exec repositories.Executor, tagId string) error {
	args := r.Called(exec, tagId)
	return args.Error(0)
}
