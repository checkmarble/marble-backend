package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type CaseRepository struct {
	mock.Mock
}

func (r *CaseRepository) ListOrganizationCases(tx repositories.Transaction, organizationId string, filters models.CaseFilters, pagination models.PaginationAndSorting) ([]models.CaseWithRank, error) {
	args := r.Called(tx, organizationId)
	return args.Get(0).([]models.CaseWithRank), args.Error(1)
}

func (r *CaseRepository) GetCaseById(tx repositories.Transaction, caseId string) (models.Case, error) {
	args := r.Called(tx, caseId)
	return args.Get(0).(models.Case), args.Error(1)
}

func (r *CaseRepository) CreateCase(tx repositories.Transaction, createCaseAttributes models.CreateCaseAttributes, newCaseId string) error {
	args := r.Called(tx, createCaseAttributes, newCaseId)
	return args.Error(0)
}

func (r *CaseRepository) UpdateCase(tx repositories.Transaction, caseId string, updateCaseAttributes models.UpdateCaseAttributes) error {
	args := r.Called(tx, caseId, updateCaseAttributes)
	return args.Error(0)
}

func (r *CaseRepository) CreateCaseTag(tx repositories.Transaction, newCaseTagId string, createCaseTagAttributes models.CreateCaseTagAttributes) error {
	args := r.Called(tx, newCaseTagId, createCaseTagAttributes)
	return args.Error(0)
}

func (r *CaseRepository) GetCaseTagById(tx repositories.Transaction, caseTagId string) (models.CaseTag, error) {
	args := r.Called(tx, caseTagId)
	return args.Get(0).(models.CaseTag), args.Error(1)
}

func (r *CaseRepository) SoftDeleteCaseTag(tx repositories.Transaction, tagId string) error {
	args := r.Called(tx, tagId)
	return args.Error(0)
}
