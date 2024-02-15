package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type CaseEventRepository struct {
	mock.Mock
}

func (r *CaseEventRepository) ListCaseEvents(exec repositories.Executor, caseId string) ([]models.CaseEvent, error) {
	args := r.Called(exec, caseId)
	return args.Get(0).([]models.CaseEvent), args.Error(1)
}

func (r *CaseEventRepository) CreateCaseEvent(exec repositories.Executor,
	createCaseEventAttributes models.CreateCaseEventAttributes, newCaseEventId string,
) error {
	args := r.Called(exec, createCaseEventAttributes, newCaseEventId)
	return args.Error(0)
}

func (r *CaseEventRepository) BatchCreateCaseEvents(exec repositories.Executor,
	createCaseEventAttributes []models.CreateCaseEventAttributes,
) error {
	args := r.Called(exec, createCaseEventAttributes)
	return args.Error(0)
}
