package usecases

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"slices"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type CaseUseCaseRepository interface {
	ListOrganizationCases(ctx context.Context, exec repositories.Executor, filters models.CaseFilters,
		pagination models.PaginationAndSorting) ([]models.CaseWithRank, error)
	GetCaseById(ctx context.Context, exec repositories.Executor, caseId string) (models.Case, error)
	CreateCase(ctx context.Context, exec repositories.Executor,
		createCaseAttributes models.CreateCaseAttributes, newCaseId string) error
	UpdateCase(ctx context.Context, exec repositories.Executor,
		updateCaseAttributes models.UpdateCaseAttributes) error
	SnoozeCase(ctx context.Context, exec repositories.Executor, snoozeRequest models.CaseSnoozeRequest) error
	UnsnoozeCase(ctx context.Context, exec repositories.Executor,
		caseId string) error

	CreateCaseEvent(ctx context.Context, exec repositories.Executor,
		createCaseEventAttributes models.CreateCaseEventAttributes) error
	BatchCreateCaseEvents(ctx context.Context, exec repositories.Executor,
		createCaseEventAttributes []models.CreateCaseEventAttributes) error
	ListCaseEvents(ctx context.Context, exec repositories.Executor, caseId string) ([]models.CaseEvent, error)

	GetCaseContributor(ctx context.Context, exec repositories.Executor, caseId, userId string) (*models.CaseContributor, error)
	CreateCaseContributor(ctx context.Context, exec repositories.Executor, caseId, userId string) error

	GetTagById(ctx context.Context, exec repositories.Executor, tagId string) (models.Tag, error)
	CreateCaseTag(ctx context.Context, exec repositories.Executor, caseId, tagId string) error
	ListCaseTagsByCaseId(ctx context.Context, exec repositories.Executor, caseId string) ([]models.CaseTag, error)
	SoftDeleteCaseTag(ctx context.Context, exec repositories.Executor, tagId string) error

	CreateDbCaseFile(ctx context.Context, exec repositories.Executor,
		createCaseFileInput models.CreateDbCaseFileInput) error
	GetCaseFileById(ctx context.Context, exec repositories.Executor, caseFileId string) (models.CaseFile, error)
	GetCasesFileByCaseId(ctx context.Context, exec repositories.Executor, caseId string) ([]models.CaseFile, error)
}

type CaseUsecaseSanctionCheckRepository interface {
	GetActiveSanctionCheckForDecision(context.Context, repositories.Executor, string) (
		*models.SanctionCheckWithMatches, error)
}

type webhookEventsUsecase interface {
	CreateWebhookEvent(
		ctx context.Context,
		tx repositories.Transaction,
		input models.WebhookEventCreate,
	) error
	SendWebhookEventAsync(ctx context.Context, webhookEventId string)
}

type CaseUseCase struct {
	enforceSecurity         security.EnforceSecurityCase
	repository              CaseUseCaseRepository
	decisionRepository      repositories.DecisionRepository
	inboxReader             inboxes.InboxReader
	blobRepository          repositories.BlobRepository
	caseManagerBucketUrl    string
	transactionFactory      executor_factory.TransactionFactory
	executorFactory         executor_factory.ExecutorFactory
	webhookEventsUsecase    webhookEventsUsecase
	sanctionCheckRepository CaseUsecaseSanctionCheckRepository
}

func (usecase *CaseUseCase) ListCases(
	ctx context.Context,
	organizationId string,
	pagination models.PaginationAndSorting,
	filters dto.CaseFilters,
) (models.CaseListPage, error) {
	if !filters.StartDate.IsZero() && !filters.EndDate.IsZero() &&
		filters.StartDate.After(filters.EndDate) {
		return models.CaseListPage{}, fmt.Errorf("start date must be before end date: %w", models.BadParameterError)
	}
	statuses, err := models.ValidateCaseStatuses(filters.Statuses)
	if err != nil {
		return models.CaseListPage{}, err
	}

	if err := models.ValidatePagination(pagination); err != nil {
		return models.CaseListPage{}, err
	}

	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.CaseListPage, error) {
			availableInboxIds, err := usecase.getAvailableInboxIds(ctx, tx, organizationId)
			if err != nil {
				return models.CaseListPage{}, err
			}
			if len(filters.InboxIds) > 0 {
				for _, inboxId := range filters.InboxIds {
					if !slices.Contains(availableInboxIds, inboxId) {
						return models.CaseListPage{}, errors.Wrap(
							models.ForbiddenError, fmt.Sprintf("inbox %s is not accessible", inboxId))
					}
				}
			}

			repoFilters := models.CaseFilters{
				StartDate:      filters.StartDate,
				EndDate:        filters.EndDate,
				Statuses:       statuses,
				OrganizationId: organizationId,
				Name:           filters.Name,
				IncludeSnoozed: filters.IncludeSnoozed,
			}
			if len(filters.InboxIds) > 0 {
				repoFilters.InboxIds = filters.InboxIds
			} else {
				repoFilters.InboxIds = availableInboxIds
			}

			paginationWithOneMore := pagination
			paginationWithOneMore.Limit++

			cases, err := usecase.repository.ListOrganizationCases(ctx, tx, repoFilters, paginationWithOneMore)
			if err != nil {
				return models.CaseListPage{}, err
			}
			for _, c := range cases {
				if err := usecase.enforceSecurity.ReadOrUpdateCase(c.Case, availableInboxIds); err != nil {
					return models.CaseListPage{}, err
				}
			}

			if len(cases) == 0 {
				return models.CaseListPage{}, nil
			}

			hasNextPage := len(cases) > pagination.Limit
			if hasNextPage {
				cases = cases[:len(cases)-1]
			}

			casesWithoutRank := make([]models.Case, len(cases))
			for i, c := range cases {
				casesWithoutRank[i] = c.Case
			}

			return models.CaseListPage{
				Cases:       casesWithoutRank,
				StartIndex:  cases[0].RankNumber,
				EndIndex:    cases[len(cases)-1].RankNumber,
				HasNextPage: hasNextPage,
			}, nil
		},
	)
}

func (usecase *CaseUseCase) getAvailableInboxIds(ctx context.Context, exec repositories.Executor, organizationId string) ([]string, error) {
	inboxes, err := usecase.inboxReader.ListInboxes(ctx, exec, organizationId, false)
	if err != nil {
		return []string{}, errors.Wrap(err, "failed to list available inboxes in usecase")
	}
	availableInboxIds := make([]string, len(inboxes))
	for i, inbox := range inboxes {
		availableInboxIds[i] = inbox.Id
	}
	return availableInboxIds, nil
}

func (usecase *CaseUseCase) GetCase(ctx context.Context, caseId string) (models.Case, error) {
	exec := usecase.executorFactory.NewExecutor()
	c, err := usecase.getCaseWithDetails(ctx, exec, caseId)
	if err != nil {
		return models.Case{}, err
	}

	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, c.OrganizationId)
	if err != nil {
		return models.Case{}, err
	}
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
		return models.Case{}, err
	}

	return c, nil
}

func (usecase *CaseUseCase) CreateCase(
	ctx context.Context,
	tx repositories.Transaction,
	userId string,
	createCaseAttributes models.CreateCaseAttributes,
	fromEndUser bool,
) (models.Case, error) {
	if err := usecase.validateDecisions(ctx, tx, createCaseAttributes.DecisionIds); err != nil {
		return models.Case{}, err
	}
	newCaseId := uuid.NewString()
	err := usecase.repository.CreateCase(ctx, tx, createCaseAttributes, newCaseId)
	if err != nil {
		return models.Case{}, err
	}

	if fromEndUser {
		if err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			CaseId:    newCaseId,
			UserId:    userId,
			EventType: models.CaseCreated,
		}); err != nil {
			return models.Case{}, err
		}
		if err := usecase.createCaseContributorIfNotExist(ctx, tx, newCaseId, userId); err != nil {
			return models.Case{}, err
		}

	} else {
		if err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			CaseId:    newCaseId,
			EventType: models.CaseCreated,
		}); err != nil {
			return models.Case{}, err
		}
	}

	err = usecase.UpdateDecisionsWithEvents(ctx, tx, newCaseId, userId, createCaseAttributes.DecisionIds)
	if err != nil {
		return models.Case{}, err
	}

	return usecase.getCaseWithDetails(ctx, tx, newCaseId)
}

func (usecase *CaseUseCase) CreateCaseAsUser(
	ctx context.Context,
	organizationId string,
	userId string,
	createCaseAttributes models.CreateCaseAttributes,
) (models.Case, error) {
	exec := usecase.executorFactory.NewExecutor()
	webhookEventId := uuid.NewString()
	c, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory,
		func(tx repositories.Transaction) (models.Case, error) {
			// permission check on the inbox as end user
			availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, organizationId)
			if err != nil {
				return models.Case{}, err
			}
			if err := usecase.enforceSecurity.CreateCase(createCaseAttributes, availableInboxIds); err != nil {
				return models.Case{}, err
			}

			newCase, err := usecase.CreateCase(ctx, tx, userId, createCaseAttributes, true)
			if err != nil {
				return models.Case{}, err
			}

			err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
				Id:             webhookEventId,
				OrganizationId: newCase.OrganizationId,
				EventContent:   models.NewWebhookEventCaseCreatedManually(newCase.GetMetadata()),
			})
			if err != nil {
				return models.Case{}, err
			}

			return newCase, nil
		})
	if err != nil {
		return models.Case{}, err
	}

	usecase.webhookEventsUsecase.SendWebhookEventAsync(ctx, webhookEventId)

	tracking.TrackEvent(ctx, models.AnalyticsCaseCreated, map[string]interface{}{
		"case_id": c.Id,
	})

	return c, nil
}

func (usecase *CaseUseCase) UpdateCase(
	ctx context.Context,
	userId string,
	updateCaseAttributes models.UpdateCaseAttributes,
) (models.Case, error) {
	webhookEventId := uuid.New().String()

	updatedCase, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(ctx, tx, updateCaseAttributes.Id)
		if err != nil {
			return models.Case{}, err
		}

		if isIdenticalCaseUpdate(updateCaseAttributes, c) {
			return usecase.getCaseWithDetails(ctx, tx, updateCaseAttributes.Id)
		}

		availableInboxIds, err := usecase.getAvailableInboxIds(
			ctx,
			usecase.executorFactory.NewExecutor(),
			c.OrganizationId)
		if err != nil {
			return models.Case{}, err
		}
		if updateCaseAttributes.InboxId != "" {
			// access check on the case's new requested inbox
			if _, err := usecase.inboxReader.GetInboxById(ctx,
				updateCaseAttributes.InboxId); err != nil {
				return models.Case{}, errors.Wrap(err,
					fmt.Sprintf("User does not have access the new inbox %s", updateCaseAttributes.InboxId))
			}
		}
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
			return models.Case{}, err
		}

		err = usecase.repository.UpdateCase(ctx, tx, updateCaseAttributes)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.createCaseContributorIfNotExist(ctx, tx, updateCaseAttributes.Id, userId); err != nil {
			return models.Case{}, err
		}

		if err := usecase.updateCaseCreateEvents(ctx, tx, updateCaseAttributes, c, userId); err != nil {
			return models.Case{}, err
		}

		updatedCase, err := usecase.getCaseWithDetails(ctx, tx, updateCaseAttributes.Id)
		if err != nil {
			return models.Case{}, err
		}

		err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: updatedCase.OrganizationId,
			EventContent:   models.NewWebhookEventCaseUpdated(updatedCase),
		})
		if err != nil {
			return models.Case{}, err
		}

		return updatedCase, nil
	})
	if err != nil {
		return models.Case{}, err
	}

	usecase.webhookEventsUsecase.SendWebhookEventAsync(ctx, webhookEventId)

	trackCaseUpdatedEvents(ctx, updatedCase.Id, updateCaseAttributes)
	return updatedCase, nil
}

func (uc *CaseUseCase) Snooze(ctx context.Context, req models.CaseSnoozeRequest) error {
	c, err := uc.repository.GetCaseById(ctx, uc.executorFactory.NewExecutor(), req.CaseId)
	if err != nil {
		return err
	}

	if err := uc.enforceSecurity.ReadOrUpdateCase(c, []string{c.InboxId}); err != nil {
		return err
	}

	return uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := uc.repository.SnoozeCase(ctx, tx, req); err != nil {
			return err
		}

		var previousSnooze *string

		if c.IsSnoozed() {
			previousSnooze = utils.Ptr(c.SnoozedUntil.Format(time.RFC3339))
		}

		event := models.CreateCaseEventAttributes{
			UserId:        string(req.UserId),
			CaseId:        req.CaseId,
			EventType:     models.CaseSnoozed,
			NewValue:      utils.Ptr(req.Until.Format(time.RFC3339)),
			PreviousValue: previousSnooze,
		}

		if err = uc.repository.CreateCaseEvent(ctx, tx, event); err != nil {
			return err
		}

		return nil
	})
}

func (uc *CaseUseCase) Unsnooze(ctx context.Context, req models.CaseSnoozeRequest) error {
	c, err := uc.repository.GetCaseById(ctx, uc.executorFactory.NewExecutor(), req.CaseId)
	if err != nil {
		return err
	}

	if !c.IsSnoozed() {
		return errors.Wrap(models.ConflictError, "case is not currently snoozed")
	}

	return uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := uc.enforceSecurity.ReadOrUpdateCase(c, []string{c.InboxId}); err != nil {
			return err
		}

		if err = uc.repository.UnsnoozeCase(ctx, uc.executorFactory.NewExecutor(), req.CaseId); err != nil {
			return err
		}

		event := models.CreateCaseEventAttributes{
			UserId:        string(req.UserId),
			CaseId:        req.CaseId,
			EventType:     models.CaseUnsnoozed,
			PreviousValue: utils.Ptr(c.SnoozedUntil.Format(time.RFC3339)),
		}

		if err = uc.repository.CreateCaseEvent(ctx, tx, event); err != nil {
			return err
		}

		return err
	})
}

func isIdenticalCaseUpdate(updateCaseAttributes models.UpdateCaseAttributes, c models.Case) bool {
	return (updateCaseAttributes.Name == "" || updateCaseAttributes.Name == c.Name) &&
		(updateCaseAttributes.Status == "" || updateCaseAttributes.Status == c.Status) &&
		(updateCaseAttributes.InboxId == "" || updateCaseAttributes.InboxId == c.InboxId)
}

func (usecase *CaseUseCase) updateCaseCreateEvents(ctx context.Context, exec repositories.Executor,
	updateCaseAttributes models.UpdateCaseAttributes, oldCase models.Case, userId string,
) error {
	var err error
	if updateCaseAttributes.Name != "" && updateCaseAttributes.Name != oldCase.Name {
		err = usecase.repository.CreateCaseEvent(ctx, exec, models.CreateCaseEventAttributes{
			CaseId:        updateCaseAttributes.Id,
			UserId:        userId,
			EventType:     models.CaseNameUpdated,
			NewValue:      &updateCaseAttributes.Name,
			PreviousValue: &oldCase.Name,
		})
		if err != nil {
			return err
		}
	}

	if updateCaseAttributes.Status != "" && updateCaseAttributes.Status != oldCase.Status {
		newStatus := string(updateCaseAttributes.Status)
		err = usecase.repository.CreateCaseEvent(ctx, exec, models.CreateCaseEventAttributes{
			CaseId:        updateCaseAttributes.Id,
			UserId:        userId,
			EventType:     models.CaseStatusUpdated,
			NewValue:      &newStatus,
			PreviousValue: (*string)(&oldCase.Status),
		})
		if err != nil {
			return err
		}
	}

	if updateCaseAttributes.InboxId != "" && updateCaseAttributes.InboxId != oldCase.InboxId {
		err = usecase.repository.CreateCaseEvent(ctx, exec, models.CreateCaseEventAttributes{
			CaseId:        updateCaseAttributes.Id,
			UserId:        userId,
			EventType:     models.CaseInboxChanged,
			NewValue:      &updateCaseAttributes.InboxId,
			PreviousValue: &oldCase.InboxId,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (usecase *CaseUseCase) AddDecisionsToCase(ctx context.Context, userId, caseId string, decisionIds []string) (models.Case, error) {
	webhookEventId := uuid.New().String()

	updatedCase, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(ctx, tx, caseId)
		if err != nil {
			return models.Case{}, err
		}
		availableInboxIds, err := usecase.getAvailableInboxIds(ctx, tx, c.OrganizationId)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
			return models.Case{}, err
		}
		if err := usecase.validateDecisions(ctx, tx, decisionIds); err != nil {
			return models.Case{}, err
		}

		err = usecase.UpdateDecisionsWithEvents(ctx, tx, caseId, userId, decisionIds)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.createCaseContributorIfNotExist(ctx, tx, caseId, userId); err != nil {
			return models.Case{}, err
		}

		updatedCase, err := usecase.getCaseWithDetails(ctx, tx, caseId)
		if err != nil {
			return models.Case{}, err
		}

		err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: updatedCase.OrganizationId,
			EventContent:   models.NewWebhookEventCaseDecisionsUpdated(updatedCase.GetMetadata()),
		})
		if err != nil {
			return models.Case{}, err
		}

		return updatedCase, nil
	})
	if err != nil {
		return models.Case{}, err
	}

	usecase.webhookEventsUsecase.SendWebhookEventAsync(ctx, webhookEventId)

	tracking.TrackEvent(ctx, models.AnalyticsDecisionsAdded, map[string]interface{}{
		"case_id": updatedCase.Id,
	})
	return updatedCase, nil
}

func (usecase *CaseUseCase) CreateCaseComment(ctx context.Context, userId string,
	caseCommentAttributes models.CreateCaseCommentAttributes,
) (models.Case, error) {
	webhookEventId := uuid.New().String()

	updatedCase, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(ctx, tx, caseCommentAttributes.Id)
		if err != nil {
			return models.Case{}, err
		}

		availableInboxIds, err := usecase.getAvailableInboxIds(ctx, tx, c.OrganizationId)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
			return models.Case{}, err
		}

		if err := usecase.createCaseContributorIfNotExist(ctx, tx, caseCommentAttributes.Id, userId); err != nil {
			return models.Case{}, err
		}

		err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			CaseId:         caseCommentAttributes.Id,
			UserId:         userId,
			EventType:      models.CaseCommentAdded,
			AdditionalNote: &caseCommentAttributes.Comment,
		})
		if err != nil {
			return models.Case{}, err
		}
		updatedCase, err := usecase.getCaseWithDetails(ctx, tx, caseCommentAttributes.Id)
		if err != nil {
			return models.Case{}, err
		}

		err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: updatedCase.OrganizationId,
			EventContent:   models.NewWebhookEventCaseCommentCreated(updatedCase),
		})
		if err != nil {
			return models.Case{}, err
		}

		return updatedCase, nil
	})
	if err != nil {
		return models.Case{}, err
	}

	usecase.webhookEventsUsecase.SendWebhookEventAsync(ctx, webhookEventId)

	tracking.TrackEvent(ctx, models.AnalyticsCaseCommentCreated, map[string]interface{}{
		"case_id": updatedCase.Id,
	})
	return updatedCase, nil
}

func (usecase *CaseUseCase) CreateCaseTags(ctx context.Context, userId string,
	caseTagAttributes models.CreateCaseTagsAttributes,
) (models.Case, error) {
	webhookEventId := uuid.New().String()

	updatedCase, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(ctx, tx, caseTagAttributes.CaseId)
		if err != nil {
			return models.Case{}, err
		}

		availableInboxIds, err := usecase.getAvailableInboxIds(
			ctx,
			usecase.executorFactory.NewExecutor(),
			c.OrganizationId)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
			return models.Case{}, err
		}

		previousCaseTags, err := usecase.repository.ListCaseTagsByCaseId(ctx, tx, caseTagAttributes.CaseId)
		if err != nil {
			return models.Case{}, err
		}
		previousTagIds := pure_utils.Map(previousCaseTags,
			func(caseTag models.CaseTag) string { return caseTag.TagId })

		for _, tagId := range caseTagAttributes.TagIds {
			if !slices.Contains(previousTagIds, tagId) {
				if err := usecase.createCaseTag(ctx, tx, caseTagAttributes.CaseId, tagId); err != nil {
					return models.Case{}, err
				}
			}
		}

		for _, caseTag := range previousCaseTags {
			if !slices.Contains(caseTagAttributes.TagIds, caseTag.TagId) {
				if err = usecase.repository.SoftDeleteCaseTag(ctx, tx, caseTag.Id); err != nil {
					return models.Case{}, err
				}
			}
		}

		previousValue := strings.Join(previousTagIds, ",")
		newValue := strings.Join(caseTagAttributes.TagIds, ",")
		err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			CaseId:        caseTagAttributes.CaseId,
			UserId:        userId,
			EventType:     models.CaseTagsUpdated,
			PreviousValue: &previousValue,
			NewValue:      &newValue,
		})
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.createCaseContributorIfNotExist(ctx, tx, caseTagAttributes.CaseId, userId); err != nil {
			return models.Case{}, err
		}

		updatedCase, err := usecase.getCaseWithDetails(ctx, tx, caseTagAttributes.CaseId)
		if err != nil {
			return models.Case{}, err
		}

		err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: updatedCase.OrganizationId,
			EventContent:   models.NewWebhookEventCaseTagsUpdated(updatedCase),
		})
		if err != nil {
			return models.Case{}, err
		}

		return updatedCase, nil
	})
	if err != nil {
		return models.Case{}, err
	}

	usecase.webhookEventsUsecase.SendWebhookEventAsync(ctx, webhookEventId)

	tracking.TrackEvent(ctx, models.AnalyticsCaseTagsUpdated, map[string]interface{}{
		"case_id": updatedCase.Id,
	})
	return updatedCase, nil
}

func (usecase *CaseUseCase) createCaseTag(ctx context.Context, exec repositories.Executor, caseId, tagId string) error {
	tag, err := usecase.repository.GetTagById(ctx, exec, tagId)
	if err != nil {
		return err
	}

	if tag.DeletedAt != nil {
		return fmt.Errorf("tag %s is deleted %w", tag.Id, models.BadParameterError)
	}

	if err = usecase.repository.CreateCaseTag(ctx, exec, caseId, tagId); err != nil {
		if repositories.IsUniqueViolationError(err) {
			return fmt.Errorf("tag %s already added to case %s %w", tag.Id, caseId, models.ConflictError)
		}
		return err
	}

	return nil
}

func (usecase *CaseUseCase) getCaseWithDetails(ctx context.Context, exec repositories.Executor, caseId string) (models.Case, error) {
	c, err := usecase.repository.GetCaseById(ctx, exec, caseId)
	if err != nil {
		return models.Case{}, err
	}

	decisions, err := usecase.decisionRepository.DecisionsByCaseId(ctx, exec, c.OrganizationId, caseId)
	if err != nil {
		return models.Case{}, err
	}
	c.Decisions = decisions

	caseFiles, err := usecase.repository.GetCasesFileByCaseId(ctx, exec, caseId)
	if err != nil {
		return models.Case{}, err
	}
	c.Files = caseFiles

	events, err := usecase.repository.ListCaseEvents(ctx, exec, caseId)
	if err != nil {
		return models.Case{}, err
	}
	c.Events = events

	return c, nil
}

func (usecase *CaseUseCase) validateDecisions(ctx context.Context, exec repositories.Executor, decisionIds []string) error {
	if len(decisionIds) == 0 {
		return nil
	}
	decisions, err := usecase.decisionRepository.DecisionsById(ctx, exec, decisionIds)
	if err != nil {
		return err
	}

	for _, decision := range decisions {
		if decision.Case != nil && decision.Case.Id != "" {
			return fmt.Errorf("decision %s already belongs to a case %s %w",
				decision.DecisionId, (*decision.Case).Id, models.BadParameterError)
		}
	}
	return nil
}

func (usecase *CaseUseCase) UpdateDecisionsWithEvents(
	ctx context.Context,
	exec repositories.Executor,
	caseId, userId string,
	decisionIdsToAdd []string,
) error {
	if len(decisionIdsToAdd) > 0 {
		if err := usecase.decisionRepository.UpdateDecisionCaseId(ctx, exec, decisionIdsToAdd, caseId); err != nil {
			return err
		}

		err := usecase.repository.UnsnoozeCase(ctx, exec, caseId)
		if err != nil {
			return err
		}

		createCaseEventAttributes := make([]models.CreateCaseEventAttributes, len(decisionIdsToAdd))
		resourceType := models.DecisionResourceType
		for i, decisionId := range decisionIdsToAdd {
			createCaseEventAttributes[i] = models.CreateCaseEventAttributes{
				CaseId:       caseId,
				UserId:       userId,
				EventType:    models.DecisionAdded,
				ResourceId:   &decisionId,
				ResourceType: &resourceType,
			}
		}
		if err := usecase.repository.BatchCreateCaseEvents(ctx, exec,
			createCaseEventAttributes); err != nil {
			return err
		}
	}
	return nil
}

func (usecase *CaseUseCase) createCaseContributorIfNotExist(ctx context.Context, exec repositories.Executor, caseId, userId string) error {
	contributor, err := usecase.repository.GetCaseContributor(ctx, exec, caseId, userId)
	if err != nil {
		return err
	}
	if contributor != nil {
		return nil
	}
	return usecase.repository.CreateCaseContributor(ctx, exec, caseId, userId)
}

func trackCaseUpdatedEvents(ctx context.Context, caseId string, updateCaseAttributes models.UpdateCaseAttributes) {
	if updateCaseAttributes.Status != "" {
		tracking.TrackEvent(ctx, models.AnalyticsCaseStatusUpdated, map[string]interface{}{
			"case_id": caseId,
		})
	}
	if updateCaseAttributes.Name != "" {
		tracking.TrackEvent(ctx, models.AnalyticsCaseUpdated, map[string]interface{}{
			"case_id": caseId,
		})
	}
}

func (usecase *CaseUseCase) CreateCaseFiles(ctx context.Context, input models.CreateCaseFilesInput) (models.Case, error) {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	creds, found := utils.CredentialsFromCtx(ctx)
	if !found {
		return models.Case{}, errors.New("no credentials in context")
	}
	userId := string(creds.ActorIdentity.UserId)

	for _, fileHeader := range input.Files {
		if err := validateFileType(fileHeader); err != nil {
			return models.Case{}, err
		}
	}

	// permissions check
	c, err := usecase.repository.GetCaseById(ctx, exec, input.CaseId)
	if err != nil {
		return models.Case{}, err
	}
	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, c.OrganizationId)
	if err != nil {
		return models.Case{}, err
	}
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
		return models.Case{}, err
	}

	type uploadedFileMetadata struct {
		fileReference string
		fileName      string
	}
	uploadedFilesMetadata := make([]uploadedFileMetadata, 0, len(input.Files))
	for _, fileHeader := range input.Files {
		newFileReference := fmt.Sprintf("%s/%s/%s", creds.OrganizationId, input.CaseId, uuid.NewString())
		err = writeToBlobStorage(ctx, usecase, fileHeader, newFileReference)
		if err != nil {
			break
		}

		uploadedFilesMetadata = append(uploadedFilesMetadata, uploadedFileMetadata{
			fileReference: newFileReference,
			fileName:      fileHeader.Filename,
		})
	}
	if err != nil {
		for _, uploadedFile := range uploadedFilesMetadata {
			if deleteErr := usecase.blobRepository.DeleteFile(ctx,
				usecase.caseManagerBucketUrl, uploadedFile.fileReference); deleteErr != nil {
				logger.WarnContext(ctx, fmt.Sprintf("failed to clean up blob %s after case file creation failed", uploadedFile.fileReference),
					"bucket", usecase.caseManagerBucketUrl,
					"file_reference", uploadedFile.fileReference,
					"error", deleteErr)
			}
		}
		return models.Case{}, err
	}

	webhookEventId := uuid.NewString()
	err = usecase.transactionFactory.Transaction(ctx, func(
		tx repositories.Transaction,
	) error {
		if err := usecase.createCaseContributorIfNotExist(ctx, tx, input.CaseId, userId); err != nil {
			return err
		}

		for _, uploadedFile := range uploadedFilesMetadata {
			newCaseFileId := uuid.NewString()
			if err := usecase.repository.CreateDbCaseFile(
				ctx,
				tx,
				models.CreateDbCaseFileInput{
					Id:            newCaseFileId,
					BucketName:    usecase.caseManagerBucketUrl,
					CaseId:        input.CaseId,
					FileName:      uploadedFile.fileName,
					FileReference: uploadedFile.fileReference,
				},
			); err != nil {
				return err
			}

			resourceType := models.CaseFileResourceType
			err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
				CaseId:         input.CaseId,
				UserId:         userId,
				EventType:      models.CaseFileAdded,
				ResourceType:   &resourceType,
				ResourceId:     &newCaseFileId,
				AdditionalNote: &uploadedFile.fileName,
			})
			if err != nil {
				return err
			}
		}

		// Create a single webhook event for the case
		err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: creds.OrganizationId,
			EventContent:   models.NewWebhookEventCaseFileCreated(input.CaseId),
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return models.Case{}, err
	}

	usecase.webhookEventsUsecase.SendWebhookEventAsync(ctx, webhookEventId)
	// Dispatch one event per file uploaded, so it's easier to do aggregation in the analytics
	for range uploadedFilesMetadata {
		tracking.TrackEvent(ctx, models.AnalyticsCaseFileCreated, map[string]interface{}{
			"case_id": input.CaseId,
		})
	}

	return usecase.getCaseWithDetails(ctx, exec, input.CaseId)
}

func validateFileType(file multipart.FileHeader) error {
	supportedFileTypes := []string{
		"text/",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.",
		"application/msword",
		"application/zip",
		"application/x-zip-compressed",
		"application/pdf",
		"image/",
	}
	errFileType := errors.Wrap(models.BadParameterError,
		fmt.Sprintf("file type not supported: %s", file.Header.Get("Content-Type")))
	for _, supportedFileType := range supportedFileTypes {
		if strings.HasPrefix(file.Header.Get("Content-Type"), supportedFileType) {
			return nil
		}
	}

	return errFileType
}

func writeToBlobStorage(ctx context.Context, usecase *CaseUseCase, fileHeader multipart.FileHeader, newFileReference string) error {
	writer, err := usecase.blobRepository.OpenStream(ctx, usecase.caseManagerBucketUrl, newFileReference, fileHeader.Filename)
	if err != nil {
		return err
	}
	defer writer.Close() // We should still call Close when we are finished writing to check the error if any - this is a no-op if Close has already been called

	file, err := fileHeader.Open()
	if err != nil {
		return errors.Wrap(models.BadParameterError, err.Error())
	}
	if _, err := io.Copy(writer, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	return nil
}

func (usecase *CaseUseCase) GetCaseFileUrl(ctx context.Context, caseFileId string) (string, error) {
	exec := usecase.executorFactory.NewExecutor()
	cf, err := usecase.repository.GetCaseFileById(ctx, exec, caseFileId)
	if err != nil {
		return "", err
	}

	c, err := usecase.getCaseWithDetails(ctx, exec, cf.CaseId)
	if err != nil {
		return "", err
	}
	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, c.OrganizationId)
	if err != nil {
		return "", err
	}
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
		return "", err
	}

	return usecase.blobRepository.GenerateSignedUrl(ctx, usecase.caseManagerBucketUrl, cf.FileReference)
}

func (usecase *CaseUseCase) CreateRuleSnoozeEvent(ctx context.Context, tx repositories.Transaction, input models.RuleSnoozeCaseEventInput,
) error {
	c, err := usecase.repository.GetCaseById(ctx, tx, input.CaseId)
	if err != nil {
		return err
	}

	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, tx, c.OrganizationId)
	if err != nil {
		return err
	}
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
		return err
	}

	if err := usecase.createCaseContributorIfNotExist(ctx, tx, input.CaseId, input.UserId); err != nil {
		return err
	}

	resourceType := models.RuleSnoozeResourceType
	err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
		AdditionalNote: &input.Comment,
		CaseId:         input.CaseId,
		UserId:         input.UserId,
		EventType:      models.CaseRuleSnoozeCreated,
		ResourceType:   &resourceType,
		ResourceId:     &input.RuleSnoozeId,
	})
	if err != nil {
		return err
	}
	updatedCase, err := usecase.getCaseWithDetails(ctx, tx, input.CaseId)
	if err != nil {
		return err
	}

	err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
		Id:             input.WebhookEventId,
		OrganizationId: updatedCase.OrganizationId,
		EventContent:   models.NewWebhookEventCaseCommentCreated(updatedCase),
	})
	if err != nil {
		return err
	}
	return nil
}

func (usecase *CaseUseCase) ReviewCaseDecisions(
	ctx context.Context,
	input models.ReviewCaseDecisionsBody,
) (models.Case, error) {
	if !slices.Contains(models.ValidReviewStatuses, input.ReviewStatus) {
		return models.Case{}, fmt.Errorf("invalid review status %w", models.BadParameterError)
	}

	exec := usecase.executorFactory.NewExecutor()
	decisions, err := usecase.decisionRepository.DecisionsById(ctx, exec, []string{input.DecisionId})
	if err != nil {
		return models.Case{}, err
	} else if len(decisions) == 0 {
		return models.Case{}, errors.Wrapf(models.NotFoundError, "decision %s not found", input.DecisionId)
	}
	decision := decisions[0]

	err = validateDecisionReview(decision)
	if err != nil {
		return models.Case{}, err
	}
	caseId := decision.Case.Id

	c, err := usecase.repository.GetCaseById(ctx, exec, caseId)
	if err != nil {
		return models.Case{}, err
	}
	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, c.OrganizationId)
	if err != nil {
		return models.Case{}, err
	}
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
		return models.Case{}, err
	}

	if input.ReviewStatus == models.ReviewStatusApprove {
		sanctionCheck, err := usecase.sanctionCheckRepository.GetActiveSanctionCheckForDecision(ctx, exec, decisions[0].DecisionId)
		if err != nil {
			return models.Case{}, errors.Wrap(err, "could not retrieve sanction check")
		}
		if sanctionCheck != nil && sanctionCheck.Status != models.SanctionStatusNoHit {
			return models.Case{}, errors.Wrap(models.BadParameterError,
				"cannot approve a decision with possible sanction hits")
		}
	}

	webhookEventId := uuid.NewString()
	c, err = executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.Case, error) {
			err := usecase.decisionRepository.ReviewDecision(ctx, tx, input.DecisionId, input.ReviewStatus)
			if err != nil {
				return models.Case{}, err
			}

			resourceType := models.DecisionResourceType
			err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
				CaseId:         caseId,
				UserId:         input.UserId,
				EventType:      models.DecisionReviewed,
				ResourceId:     &input.DecisionId,
				ResourceType:   &resourceType,
				AdditionalNote: &input.ReviewComment,
				NewValue:       &input.ReviewStatus,
				PreviousValue:  decisions[0].ReviewStatus,
			})
			if err != nil {
				return models.Case{}, err
			}

			c, err = usecase.getCaseWithDetails(ctx, tx, caseId)
			if err != nil {
				return models.Case{}, err
			}

			err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
				Id:             webhookEventId,
				OrganizationId: c.OrganizationId,
				EventContent:   models.NewWebhookEventDecisionReviewed(c, decision.DecisionId),
			})
			if err != nil {
				return models.Case{}, err
			}

			return c, nil
		},
	)
	if err != nil {
		return models.Case{}, err
	}

	usecase.webhookEventsUsecase.SendWebhookEventAsync(ctx, webhookEventId)

	return c, nil
}

func validateDecisionReview(decision models.Decision) error {
	if decision.Case == nil {
		return errors.Wrapf(models.BadParameterError,
			"decision %s does not belong to a case", decision.DecisionId)
	}
	if decision.ReviewStatus == nil || *decision.ReviewStatus != models.ReviewStatusPending {
		return errors.Wrapf(models.BadParameterError,
			"decision %s is not in pending review", decision.DecisionId)
	}

	return nil
}
