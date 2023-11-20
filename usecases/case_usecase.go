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
	UpdateCase(tx repositories.Transaction, updateCaseAttributes models.UpdateCaseAttributes) error
	CreateCaseEvent(tx repositories.Transaction, createCaseEventAttributes models.CreateCaseEventAttributes) error
	BatchCreateCaseEvents(tx repositories.Transaction, createCaseEventAttributes []models.CreateCaseEventAttributes) error
	ListCaseEvents(tx repositories.Transaction, caseId string) ([]models.CaseEvent, error)
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
	statuses, err := models.ValidateCaseStatuses(filters.Statuses)
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

func (usecase *CaseUseCase) GetCase(caseId string) (models.Case, error) {
	c, err := usecase.getCaseWithDetails(nil, caseId)
	if err != nil {
		return models.Case{}, err
	}
	if err := usecase.enforceSecurity.ReadCase(c); err != nil {
		return models.Case{}, err
	}
	return c, nil
}

func (usecase *CaseUseCase) CreateCase(ctx context.Context, userId string, createCaseAttributes models.CreateCaseAttributes) (models.Case, error) {
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

		err = usecase.repository.CreateCaseEvent(tx, models.CreateCaseEventAttributes{
			CaseId:    newCaseId,
			UserId:    userId,
			EventType: models.CaseCreated,
		})
		if err != nil {
			return models.Case{}, err
		}

		err = usecase.updateDecisionsWithEvents(tx, newCaseId, userId, createCaseAttributes.DecisionIds)
		if err != nil {
			return models.Case{}, err
		}

		return usecase.getCaseWithDetails(tx, newCaseId)
	})

	if err != nil {
		return models.Case{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsCaseCreated, map[string]interface{}{"case_id": c.Id})

	return c, err
}

func (usecase *CaseUseCase) UpdateCase(ctx context.Context, userId string, updateCaseAttributes models.UpdateCaseAttributes) (models.Case, error) {
	if err := usecase.validateDecisions(updateCaseAttributes.DecisionIds); err != nil {
		return models.Case{}, err
	}

	updatedCase, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(tx, updateCaseAttributes.Id)
		if err != nil {
			return models.Case{}, err
		}

		if err := usecase.enforceSecurity.UpdateCase(c); err != nil {
			return models.Case{}, err
		}

		err = usecase.repository.UpdateCase(tx, updateCaseAttributes)
		if err != nil {
			return models.Case{}, err
		}

		if updateCaseAttributes.Name != "" && updateCaseAttributes.Name != c.Name {
			err = usecase.repository.CreateCaseEvent(tx, models.CreateCaseEventAttributes{
				CaseId:        updateCaseAttributes.Id,
				UserId:        userId,
				EventType:     models.CaseNameUpdated,
				NewValue:      &updateCaseAttributes.Name,
				PreviousValue: &c.Name,
			})
			if err != nil {
				return models.Case{}, err
			}
		}

		if updateCaseAttributes.Status != "" && updateCaseAttributes.Status != c.Status {
			newStatus := string(updateCaseAttributes.Status)
			err = usecase.repository.CreateCaseEvent(tx, models.CreateCaseEventAttributes{
				CaseId:        updateCaseAttributes.Id,
				UserId:        userId,
				EventType:     models.CaseStatusUpdated,
				NewValue:      &newStatus,
				PreviousValue: (*string)(&c.Status),
			})
			if err != nil {
				return models.Case{}, err
			}
		}

		err = usecase.updateDecisionsWithEvents(tx, updateCaseAttributes.Id, userId, updateCaseAttributes.DecisionIds)
		if err != nil {
			return models.Case{}, err
		}

		return usecase.getCaseWithDetails(tx, updateCaseAttributes.Id)
	})
	if err != nil {
		return models.Case{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsCaseUpdated, map[string]interface{}{"case_id": updatedCase.Id})
	return updatedCase, nil
}

func (usecase *CaseUseCase) CreateCaseComment(ctx context.Context, userId string, caseCommentAttributes models.CreateCaseCommentAttributes) (models.Case, error) {
	updatedCase, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(tx, caseCommentAttributes.Id)
		if err != nil {
			return models.Case{}, err
		}

		if err := usecase.enforceSecurity.CreateCaseComment(c); err != nil {
			return models.Case{}, err
		}

		err = usecase.repository.CreateCaseEvent(tx, models.CreateCaseEventAttributes{
			CaseId:         caseCommentAttributes.Id,
			UserId:         userId,
			EventType:      models.CaseCommentAdded,
			AdditionalNote: &caseCommentAttributes.Comment,
		})
		if err != nil {
			return models.Case{}, err
		}
		return usecase.getCaseWithDetails(tx, caseCommentAttributes.Id)
	})

	if err != nil {
		return models.Case{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsCaseCommentCreated, map[string]interface{}{"case_id": updatedCase.Id})
	return updatedCase, nil
}

func (usecase *CaseUseCase) getCaseWithDetails(tx repositories.Transaction, caseId string) (models.Case, error) {
	c, err := usecase.repository.GetCaseById(tx, caseId)
	if err != nil {
		return models.Case{}, err
	}

	decisions, err := usecase.decisionRepository.DecisionsByCaseId(tx, caseId)
	if err != nil {
		return models.Case{}, err
	}
	c.Decisions = decisions

	events, err := usecase.repository.ListCaseEvents(tx, caseId)
	if err != nil {
		return models.Case{}, err
	}
	c.Events = events

	return c, nil
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
		if decision.Case.Id != "" {
			return fmt.Errorf("decision %s already belongs to a case %s %w", decision.DecisionId, (*decision.Case).Id, models.BadParameterError)
		}
	}
	return nil
}

func (usecase *CaseUseCase) updateDecisionsWithEvents(tx repositories.Transaction, caseId, userId string, decisionIds []string) error {
	if len(decisionIds) > 0 {
		if err := usecase.decisionRepository.UpdateDecisionCaseId(tx, decisionIds, caseId); err != nil {
			return err
		}

		createCaseEventAttributes := make([]models.CreateCaseEventAttributes, len(decisionIds))
		resourceType := models.DecisionResourceType
		for i, decisionId := range decisionIds {
			createCaseEventAttributes[i] = models.CreateCaseEventAttributes{
				CaseId:       caseId,
				UserId:       userId,
				EventType:    models.DecisionAdded,
				ResourceId:   &decisionId,
				ResourceType: &resourceType,
			}
		}
		if err := usecase.repository.BatchCreateCaseEvents(tx, createCaseEventAttributes); err != nil {
			return err
		}
	}
	return nil
}
