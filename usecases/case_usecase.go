package usecases

import (
	"context"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/analytics"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type CaseUseCaseRepository interface {
	ListOrganizationCases(tx repositories.Transaction, filters models.CaseFilters, pagination models.PaginationAndSorting) ([]models.CaseWithRank, error)
	GetCaseById(tx repositories.Transaction, caseId string) (models.Case, error)
	CreateCase(tx repositories.Transaction, createCaseAttributes models.CreateCaseAttributes, newCaseId string) error
	UpdateCase(tx repositories.Transaction, updateCaseAttributes models.UpdateCaseAttributes) error
	CreateCaseEvent(tx repositories.Transaction, createCaseEventAttributes models.CreateCaseEventAttributes) error
	BatchCreateCaseEvents(tx repositories.Transaction, createCaseEventAttributes []models.CreateCaseEventAttributes) error
	ListCaseEvents(tx repositories.Transaction, caseId string) ([]models.CaseEvent, error)
	GetCaseContributor(tx repositories.Transaction, caseId, userId string) (*models.CaseContributor, error)
	CreateCaseContributor(tx repositories.Transaction, caseId, userId string) error
	GetTagById(tx repositories.Transaction, tagId string) (models.Tag, error)
	CreateCaseTag(tx repositories.Transaction, newCaseTagId string, createCaseTagAttributes models.CreateCaseTagAttributes) error
	GetCaseTagById(tx repositories.Transaction, caseTagId string) (models.CaseTag, error)
	SoftDeleteCaseTag(tx repositories.Transaction, tagId string) error
}

type CaseUseCase struct {
	enforceSecurity    security.EnforceSecurityCase
	transactionFactory transaction.TransactionFactory
	repository         CaseUseCaseRepository
	decisionRepository repositories.DecisionRepository
	inboxReader        inboxes.InboxReader
}

func (usecase *CaseUseCase) ListCases(
	ctx context.Context,
	organizationId string,
	pagination models.PaginationAndSorting,
	filters dto.CaseFilters,
) ([]models.CaseWithRank, error) {
	if !filters.StartDate.IsZero() && !filters.EndDate.IsZero() && filters.StartDate.After(filters.EndDate) {
		return []models.CaseWithRank{}, fmt.Errorf("start date must be before end date: %w", models.BadParameterError)
	}
	statuses, err := models.ValidateCaseStatuses(filters.Statuses)
	if err != nil {
		return []models.CaseWithRank{}, err
	}

	if err := models.ValidatePaginationOffset(pagination); err != nil {
		return []models.CaseWithRank{}, err
	}

	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.CaseWithRank, error) {
			inboxes, err := usecase.inboxReader.ListInboxes(ctx, tx)
			if err != nil {
				return []models.CaseWithRank{}, errors.Wrap(err, "failed to list available inboxes in usecase")
			}
			availableInboxIds := make([]string, len(inboxes))
			for i, inbox := range inboxes {
				availableInboxIds[i] = inbox.Id
			}
			if len(filters.InboxIds) > 0 {
				for _, inboxId := range filters.InboxIds {
					if !slices.Contains(availableInboxIds, inboxId) {
						return []models.CaseWithRank{}, errors.Wrap(models.ForbiddenError, fmt.Sprintf("inbox %s is not accessible", inboxId))
					}
				}
			}

			repoFilters := models.CaseFilters{
				StartDate:      filters.StartDate,
				EndDate:        filters.EndDate,
				Statuses:       statuses,
				OrganizationId: organizationId,
			}
			if len(filters.InboxIds) > 0 {
				repoFilters.InboxIds = filters.InboxIds
			} else {
				repoFilters.InboxIds = availableInboxIds
			}

			cases, err := usecase.repository.ListOrganizationCases(tx, repoFilters, pagination)
			if err != nil {
				return []models.CaseWithRank{}, err
			}
			for _, c := range cases {
				if err := usecase.enforceSecurity.ReadCase(c.Case, availableInboxIds); err != nil {
					return []models.CaseWithRank{}, err
				}
			}
			return cases, nil
		},
	)
}

func (usecase *CaseUseCase) GetCase(ctx context.Context, caseId string) (models.Case, error) {
	c, err := usecase.getCaseWithDetails(nil, caseId)
	if err != nil {
		return models.Case{}, err
	}

	// passing c.InboxId directly here because the security is already checked below in GetInboxById
	if err := usecase.enforceSecurity.ReadCase(c, []string{c.InboxId}); err != nil {
		return models.Case{}, err
	}

	// access check on the case's inbox
	_, err = usecase.inboxReader.GetInboxById(ctx, c.InboxId)
	if err != nil {
		return models.Case{}, err
	}

	return c, nil
}

func (usecase *CaseUseCase) CreateCase(ctx context.Context, userId string, createCaseAttributes models.CreateCaseAttributes) (models.Case, error) {
	if err := usecase.enforceSecurity.CreateCase(); err != nil {
		return models.Case{}, err
	}
	if _, err := usecase.inboxReader.GetInboxById(ctx, createCaseAttributes.InboxId); err != nil {
		return models.Case{}, err
	}

	c, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Case, error) {
		if err := usecase.validateDecisions(tx, createCaseAttributes.DecisionIds); err != nil {
			return models.Case{}, err
		}
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
		if err := usecase.createCaseContributorIfNotExist(tx, newCaseId, userId); err != nil {
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
	updatedCase, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(tx, updateCaseAttributes.Id)
		if err != nil {
			return models.Case{}, err
		}
		if _, err := usecase.inboxReader.GetInboxById(ctx, c.InboxId); err != nil {
			return models.Case{}, err
		}

		if err := usecase.enforceSecurity.UpdateCase(c); err != nil {
			return models.Case{}, err
		}

		err = usecase.repository.UpdateCase(tx, updateCaseAttributes)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.createCaseContributorIfNotExist(tx, updateCaseAttributes.Id, userId); err != nil {
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

	trackCaseUpdatedEvents(ctx, updatedCase.Id, updateCaseAttributes)
	return updatedCase, nil
}

func (usecase *CaseUseCase) AddDecisionsToCase(ctx context.Context, userId, caseId string, decisionIds []string) (models.Case, error) {
	updatedCase, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(tx, caseId)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.enforceSecurity.UpdateCase(c); err != nil {
			return models.Case{}, err
		}
		if err := usecase.validateDecisions(tx, decisionIds); err != nil {
			return models.Case{}, err
		}
		err = usecase.updateDecisionsWithEvents(tx, caseId, userId, decisionIds)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.createCaseContributorIfNotExist(tx, caseId, userId); err != nil {
			return models.Case{}, err
		}

		return usecase.getCaseWithDetails(tx, caseId)
	})

	if err != nil {
		return models.Case{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsDecisionsAdded, map[string]interface{}{"case_id": updatedCase.Id})
	return updatedCase, nil
}

func (usecase *CaseUseCase) CreateCaseComment(ctx context.Context, userId string, caseCommentAttributes models.CreateCaseCommentAttributes) (models.Case, error) {
	updatedCase, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(tx, caseCommentAttributes.Id)
		if err != nil {
			return models.Case{}, err
		}
		if _, err := usecase.inboxReader.GetInboxById(ctx, c.InboxId); err != nil {
			return models.Case{}, err
		}

		if err := usecase.enforceSecurity.CreateCaseComment(c); err != nil {
			return models.Case{}, err
		}
		if err := usecase.createCaseContributorIfNotExist(tx, caseCommentAttributes.Id, userId); err != nil {
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

func (usecase *CaseUseCase) CreateCaseTag(ctx context.Context, userId string, caseTagAttributes models.CreateCaseTagAttributes) (models.Case, error) {
	updatedCase, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(tx, caseTagAttributes.CaseId)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.enforceSecurity.UpdateCase(c); err != nil {
			return models.Case{}, err
		}
		tag, err := usecase.repository.GetTagById(tx, caseTagAttributes.TagId)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.enforceSecurity.ReadOrganization(tag.OrganizationId); err != nil {
			return models.Case{}, err
		}

		if tag.DeletedAt != nil {
			return models.Case{}, fmt.Errorf("tag %s is deleted %w", tag.Id, models.BadParameterError)
		}

		newCaseTagId := uuid.NewString()
		if err = usecase.repository.CreateCaseTag(tx, newCaseTagId, caseTagAttributes); err != nil {
			if repositories.IsUniqueViolationError(err) {
				return models.Case{}, fmt.Errorf("tag %s already added to case %s %w", tag.Id, c.Id, models.DuplicateValueError)
			}
			return models.Case{}, err
		}

		resourceType := models.CaseTagResourceType
		err = usecase.repository.CreateCaseEvent(tx, models.CreateCaseEventAttributes{
			CaseId:       caseTagAttributes.CaseId,
			UserId:       userId,
			EventType:    models.CaseTagAdded,
			ResourceId:   &newCaseTagId,
			ResourceType: &resourceType,
		})
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.createCaseContributorIfNotExist(tx, caseTagAttributes.CaseId, userId); err != nil {
			return models.Case{}, err
		}

		return usecase.getCaseWithDetails(tx, caseTagAttributes.CaseId)
	})

	if err != nil {
		return models.Case{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsCaseTagAdded, map[string]interface{}{"case_id": updatedCase.Id})
	return updatedCase, nil
}

func (usecase *CaseUseCase) DeleteCaseTag(ctx context.Context, userId, caseTagId string) (models.Case, error) {
	updatedCase, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Case, error) {
		caseTag, err := usecase.repository.GetCaseTagById(tx, caseTagId)
		if err != nil {
			return models.Case{}, err
		}
		c, err := usecase.repository.GetCaseById(tx, caseTag.CaseId)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.enforceSecurity.UpdateCase(c); err != nil {
			return models.Case{}, err
		}

		if err = usecase.repository.SoftDeleteCaseTag(tx, caseTagId); err != nil {
			return models.Case{}, err
		}

		resourceType := models.CaseTagResourceType
		err = usecase.repository.CreateCaseEvent(tx, models.CreateCaseEventAttributes{
			CaseId:       caseTag.CaseId,
			UserId:       userId,
			EventType:    models.CaseTagDeleted,
			ResourceId:   &caseTagId,
			ResourceType: &resourceType,
		})
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.createCaseContributorIfNotExist(tx, caseTag.CaseId, userId); err != nil {
			return models.Case{}, err
		}

		return usecase.getCaseWithDetails(tx, caseTag.CaseId)
	})

	if err != nil {
		return models.Case{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsCaseTagDeleted, map[string]interface{}{"case_id": updatedCase.Id})
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

func (usecase *CaseUseCase) validateDecisions(tx repositories.Transaction, decisionIds []string) error {
	if len(decisionIds) == 0 {
		return nil
	}
	decisions, err := usecase.decisionRepository.DecisionsById(tx, decisionIds)
	if err != nil {
		return err
	}

	for _, decision := range decisions {
		if decision.Case != nil && decision.Case.Id != "" {
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

func (usecase *CaseUseCase) createCaseContributorIfNotExist(tx repositories.Transaction, caseId, userId string) error {
	contributor, err := usecase.repository.GetCaseContributor(tx, caseId, userId)
	if err != nil {
		return err
	}
	if contributor != nil {
		return nil
	}
	return usecase.repository.CreateCaseContributor(tx, caseId, userId)
}

func trackCaseUpdatedEvents(ctx context.Context, caseId string, updateCaseAttributes models.UpdateCaseAttributes) {
	if updateCaseAttributes.Status != "" {
		analytics.TrackEvent(ctx, models.AnalyticsCaseStatusUpdated, map[string]interface{}{"case_id": caseId})
	}
	if updateCaseAttributes.Name != "" {
		analytics.TrackEvent(ctx, models.AnalyticsCaseUpdated, map[string]interface{}{"case_id": caseId})
	}
}
