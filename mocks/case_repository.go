package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type CaseRepository struct {
	mock.Mock
}

func (r *CaseRepository) ListOrganizationCases(tx repositories.Transaction_deprec, organizationId string, filters models.CaseFilters, pagination models.PaginationAndSorting) ([]models.CaseWithRank, error) {
	args := r.Called(tx, organizationId)
	return args.Get(0).([]models.CaseWithRank), args.Error(1)
}

func (r *CaseRepository) GetCaseById(tx repositories.Transaction_deprec, caseId string) (models.Case, error) {
	args := r.Called(tx, caseId)
	return args.Get(0).(models.Case), args.Error(1)
}

func (r *CaseRepository) CreateCase(tx repositories.Transaction_deprec, createCaseAttributes models.CreateCaseAttributes, newCaseId string) error {
	args := r.Called(tx, createCaseAttributes, newCaseId)
	return args.Error(0)
}

func (r *CaseRepository) UpdateCase(tx repositories.Transaction_deprec, caseId string, updateCaseAttributes models.UpdateCaseAttributes) error {
	args := r.Called(tx, caseId, updateCaseAttributes)
	return args.Error(0)
}

func (r *CaseRepository) CreateCaseTag(tx repositories.Transaction_deprec, newCaseTagId string, createCaseTagAttributes models.CreateCaseTagsAttributes) error {
	args := r.Called(tx, newCaseTagId, createCaseTagAttributes)
	return args.Error(0)
}

func (r *CaseRepository) ListCaseTagsByCaseId(tx repositories.Transaction_deprec, caseId string) ([]models.CaseTag, error) {
	args := r.Called(tx, caseId)
	return args.Get(0).([]models.CaseTag), args.Error(1)
}

func (r *CaseRepository) ListCaseTagsByTagId(tx repositories.Transaction_deprec, tagId string) ([]models.CaseTag, error) {
	args := r.Called(tx, tagId)
	return args.Get(0).([]models.CaseTag), args.Error(1)
}

func (r *CaseRepository) SoftDeleteCaseTag(tx repositories.Transaction_deprec, tagId string) error {
	args := r.Called(tx, tagId)
	return args.Error(0)
}
