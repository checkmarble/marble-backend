package usecases

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/google/uuid"
)

type CaseUseCaseRepository interface {
	ListOrganizationCases(tx repositories.Transaction, organizationId string) ([]models.Case, error)
	GetCaseById(tx repositories.Transaction, caseId string) (models.Case, error)
	CreateCase(tx repositories.Transaction, createCaseAttributes models.CreateCaseAttributes, newCaseId string) error
}

type CaseUseCase struct {
	enforceSecurity    security.EnforceSecurityCase
	transactionFactory transaction.TransactionFactory
	repository         CaseUseCaseRepository
	decisionRepository repositories.DecisionRepository
}

func (usecase *CaseUseCase) ListCases(organizationId string) ([]models.Case, error) {
	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.Case, error) {
			cases, err := usecase.repository.ListOrganizationCases(tx, organizationId)
			if err != nil {
				return []models.Case{}, err
			}
			for _, c := range cases {
				if err := usecase.enforceSecurity.ReadCase(c); err != nil {
					return []models.Case{}, err
				}
			}
			return cases, nil
		},
	)
}

func (usecase *CaseUseCase) GetCase(caseId string) (models.Case, error) {
	c, err := usecase.repository.GetCaseById(nil, caseId)
	if err != nil {
		return models.Case{}, err
	}
	if err := usecase.enforceSecurity.ReadCase(c); err != nil {
		return models.Case{}, err
	}
	return c, nil
}

func (usecase *CaseUseCase) CreateCase(createCaseAttributes models.CreateCaseAttributes) (models.Case, error) {
	if err := usecase.enforceSecurity.CreateCase(); err != nil {
		return models.Case{}, err
	}
	if err := usecase.validateDecisions(createCaseAttributes.DecisionIds); err != nil {
		return models.Case{}, err
	}

	return transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Case, error) {
		newCaseId := uuid.NewString()
		err := usecase.repository.CreateCase(tx, createCaseAttributes, newCaseId)
		if err != nil {
			return models.Case{}, err
		}
		if len(createCaseAttributes.DecisionIds) > 0 {
			if err := usecase.decisionRepository.UpdateDecisionCaseId(tx, createCaseAttributes.DecisionIds, newCaseId); err != nil {
				return models.Case{}, err
			}
		}

		return usecase.repository.GetCaseById(tx, newCaseId)
	})
}

func (usecase *CaseUseCase) validateDecisions(decisionIds []string) error {
	if len(decisionIds) == 0 {
		return nil
	}
	decisions, err := usecase.decisionRepository.DecisionsById(nil, decisionIds)
	if err != nil {
		return err
	}

	for _, decision := range decisions {
		if decision.CaseId != nil {
			caseId := *decision.CaseId
			return fmt.Errorf("decision %s already belongs to a case %s %w", decision.DecisionId, caseId, models.BadParameterError)
		}
	}
	return nil
}
