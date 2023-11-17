package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type CaseEventRepository struct {
	mock.Mock
}

func (r *CaseEventRepository) ListCaseEvents(tx repositories.Transaction, caseId string) ([]models.CaseEvent, error) {
	args := r.Called(tx, caseId)
	return args.Get(0).([]models.CaseEvent), args.Error(1)
}

func (r *CaseEventRepository) CreateCaseEvent(tx repositories.Transaction, createCaseEventAttributes models.CreateCaseEventAttributes, newCaseEventId string) error {
	args := r.Called(tx, createCaseEventAttributes, newCaseEventId)
	return args.Error(0)
}

func (r *CaseEventRepository) BatchCreateCaseEvents(tx repositories.Transaction, createCaseEventAttributes []models.CreateCaseEventAttributes) error {
	args := r.Called(tx, createCaseEventAttributes)
	return args.Error(0)
}
