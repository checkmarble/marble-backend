package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/analytics"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/google/uuid"
)

type CaseUseCaseRepository interface {
	ListOrganizationCases(tx repositories.Transaction, organizationId string, filters models.CaseFilters) ([]models.Case, error)
	GetCaseById(tx repositories.Transaction, caseId string) (models.Case, error)
	CreateCase(tx repositories.Transaction, createCaseAttributes models.CreateCaseAttributes, newCaseId string) error
}

type CaseUseCase struct {
	enforceSecurity    security.EnforceSecurityCase
	transactionFactory transaction.TransactionFactory
	repository         CaseUseCaseRepository
	decisionRepository repositories.DecisionRepository
}

func (usecase *CaseUseCase) ListCases(organizationId string, filters dto.CaseFilters) ([]models.Case, error) {
	if !filters.StartDate.IsZero() && !filters.EndDate.IsZero() && filters.StartDate.After(filters.EndDate) {
		return []models.Case{}, fmt.Errorf("start date must be before end date: %w", models.BadParameterError)
	}
	statuses, err := usecase.validateStatuses(filters.Statuses)
	if err != nil {
		return []models.Case{}, err
	}

	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.Case, error) {
			cases, err := usecase.repository.ListOrganizationCases(tx, organizationId, models.CaseFilters{
				StartDate: filters.StartDate,
				EndDate:   filters.EndDate,
				Statuses:  statuses,
			})
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

func (usecase *CaseUseCase) validateStatuses(filterStatuses []string) ([]models.CaseStatus, error) {
	statuses := make([]models.CaseStatus, len(filterStatuses))
	for i, status := range filterStatuses {
		statuses[i] = models.CaseStatusFrom(status)
		if statuses[i] == models.CaseUnknownStatus {
			return []models.CaseStatus{}, fmt.Errorf("invalid status: %s %w", status, models.BadParameterError)
		}
	}
	return statuses, nil
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

func (usecase *CaseUseCase) CreateCase(ctx context.Context, createCaseAttributes models.CreateCaseAttributes) (models.Case, error) {
	if err := usecase.enforceSecurity.CreateCase(); err != nil {
		return models.Case{}, err
	}
	if err := usecase.validateDecisions(createCaseAttributes.DecisionIds); err != nil {
		return models.Case{}, err
	}

	c, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Case, error) {
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

	if err != nil {
		return models.Case{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsCaseCreated, map[string]interface{}{"case_id": c.Id})

	return c, err
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
