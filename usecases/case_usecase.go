package usecases

import (
	"context"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"slices"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/feature_access"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
)

type CaseUseCaseRepository interface {
	ListOrganizationCases(ctx context.Context, exec repositories.Executor, filters models.CaseFilters,
		pagination models.PaginationAndSorting) ([]models.Case, error)
	GetCaseById(ctx context.Context, exec repositories.Executor, caseId string) (models.Case, error)
	GetCaseMetadataById(ctx context.Context, exec repositories.Executor, caseId string) (models.CaseMetadata, error)
	CreateCase(ctx context.Context, exec repositories.Executor,
		createCaseAttributes models.CreateCaseAttributes, newCaseId string) error
	UpdateCase(ctx context.Context, exec repositories.Executor,
		updateCaseAttributes models.UpdateCaseAttributes) error
	SnoozeCase(ctx context.Context, exec repositories.Executor, snoozeRequest models.CaseSnoozeRequest) error
	UnsnoozeCase(ctx context.Context, exec repositories.Executor,
		caseId string) error
	GetCaseReferents(ctx context.Context, exec repositories.Executor, caseIds []string) ([]models.CaseReferents, error)

	DecisionPivotValuesByCase(ctx context.Context, exec repositories.Executor, caseId string) ([]models.PivotDataWithCount, error)
	DecisionsById(ctx context.Context, exec repositories.Executor, decisionIds []string) ([]models.Decision, error)

	CreateCaseEvent(ctx context.Context, exec repositories.Executor,
		createCaseEventAttributes models.CreateCaseEventAttributes) (models.CaseEvent, error)
	BatchCreateCaseEvents(ctx context.Context, exec repositories.Executor,
		createCaseEventAttributes []models.CreateCaseEventAttributes) ([]models.CaseEvent, error)
	ListCaseEvents(ctx context.Context, exec repositories.Executor, caseId string) ([]models.CaseEvent, error)
	ListCaseEventsOfTypes(ctx context.Context, exec repositories.Executor, caseId string,
		types []models.CaseEventType, paging models.PaginationAndSorting) ([]models.CaseEvent, error)

	GetCaseContributor(ctx context.Context, exec repositories.Executor, caseId, userId string) (*models.CaseContributor, error)
	CreateCaseContributor(ctx context.Context, exec repositories.Executor, caseId, userId string) error

	GetTagById(ctx context.Context, exec repositories.Executor, tagId string) (models.Tag, error)
	CreateCaseTag(ctx context.Context, exec repositories.Executor, caseId, tagId string) error
	ListCaseTagsByCaseId(ctx context.Context, exec repositories.Executor, caseId string) ([]models.CaseTag, error)
	SoftDeleteCaseTag(ctx context.Context, exec repositories.Executor, tagId string) error

	CreateDbCaseFile(ctx context.Context, exec repositories.Executor,
		createCaseFileInput models.CreateDbCaseFileInput) (models.CaseFile, error)
	GetCaseFileById(ctx context.Context, exec repositories.Executor, caseFileId string) (models.CaseFile, error)
	GetCasesFileByCaseId(ctx context.Context, exec repositories.Executor, caseId string) ([]models.CaseFile, error)

	AssignCase(ctx context.Context, exec repositories.Executor, id string, userId *models.UserId) error
	UnassignCase(ctx context.Context, exec repositories.Executor, id string) error
	BoostCase(ctx context.Context, exec repositories.Executor, id string, reason models.BoostReason) error
	UnboostCase(ctx context.Context, exec repositories.Executor, id string) error

	EscalateCase(ctx context.Context, exec repositories.Executor, id, inboxId string) error

	GetCasesWithPivotValue(ctx context.Context, exec repositories.Executor,
		orgId uuid.UUID, pivotValue string) ([]models.Case, error)
	GetContinuousScreeningCasesWithObjectAttr(ctx context.Context, exec repositories.Executor,
		orgId uuid.UUID, objectType, objectId string) ([]models.Case, error)
	GetContinuousScreeningCasesByEntityIdInMatches(ctx context.Context, exec repositories.Executor,
		orgId uuid.UUID, entityId string) ([]models.Case, error)

	GetNextCase(ctx context.Context, exec repositories.Executor, c models.Case) (string, error)

	UserById(ctx context.Context, exec repositories.Executor, userId string) (models.User, error)

	GetMassCasesByIds(ctx context.Context, exec repositories.Executor, caseIds []uuid.UUID) ([]models.Case, error)
	CaseMassChangeStatus(ctx context.Context, tx repositories.Transaction, caseIds []uuid.UUID,
		status models.CaseStatus) ([]uuid.UUID, error)
	CaseMassAssign(ctx context.Context, tx repositories.Transaction, caseIds []uuid.UUID,
		assigneeId uuid.UUID) ([]uuid.UUID, error)
	CaseMassMoveToInbox(ctx context.Context, tx repositories.Transaction, caseIds []uuid.UUID, inboxId uuid.UUID) ([]uuid.UUID, error)

	// Continuous screenings
	ListContinuousScreeningsWithMatchesByCaseId(
		ctx context.Context,
		exec repositories.Executor,
		caseId string,
	) ([]models.ContinuousScreeningWithMatches, error)
	ListContinuousScreeningsByIds(ctx context.Context, exec repositories.Executor, ids []uuid.UUID) ([]models.ContinuousScreening, error)
	UpdateContinuousScreeningsCaseId(ctx context.Context, exec repositories.Executor, ids []uuid.UUID, caseId string) error
	GetContinuousScreeningConfig(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
	) (models.ContinuousScreeningConfig, error)

	// inboxes
	GetInboxById(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) (models.Inbox, error)

	GetCasesRelatedToObject(ctx context.Context, exec repositories.Executor, orgId uuid.UUID,
		objectType, objectId string) ([]models.Case, error)
}

type CaseUsecaseScreeningRepository interface {
	GetActiveScreeningForDecision(ctx context.Context, exec repositories.Executor, screeningId string) (
		models.ScreeningWithMatches, error)
	ListScreeningsForDecision(ctx context.Context, exec repositories.Executor, decisionId string,
		initialOnly bool) ([]models.ScreeningWithMatches, error)
}

type webhookEventsUsecase interface {
	CreateWebhookEvent(
		ctx context.Context,
		tx repositories.Transaction,
		input models.WebhookEventCreate,
	) error
	SendWebhookEventAsync(ctx context.Context, webhookEventId string)
}

type caseUsecaseIngestedDataReader interface {
	ReadPivotObjectsFromValues(
		ctx context.Context,
		organizationId uuid.UUID,
		values []models.PivotDataWithCount,
	) ([]models.PivotObject, error)
}

type caseUsecaseAiAgentUsecase interface {
	HasAiCaseReviewEnabled(ctx context.Context, orgId uuid.UUID) (bool, error)
}

type CaseUseCase struct {
	enforceSecurity         security.EnforceSecurityCase
	enforceSecurityDecision security.EnforceSecurityDecision
	repository              CaseUseCaseRepository
	decisionRepository      repositories.DecisionRepository
	inboxReader             inboxes.InboxReader
	blobRepository          repositories.BlobRepository
	caseManagerBucketUrl    string
	transactionFactory      executor_factory.TransactionFactory
	executorFactory         executor_factory.ExecutorFactory
	webhookEventsUsecase    webhookEventsUsecase
	screeningRepository     CaseUsecaseScreeningRepository
	ingestedDataReader      caseUsecaseIngestedDataReader
	taskQueueRepository     repositories.TaskQueueRepository
	featureAccessReader     feature_access.FeatureAccessReader
	aiAgentUsecase          caseUsecaseAiAgentUsecase
	publicApiAdapterUsecase PublicApiAdapterUsecase
}

func (usecase *CaseUseCase) ListCases(
	ctx context.Context,
	organizationId uuid.UUID,
	pagination models.PaginationAndSorting,
	filters models.CaseFilters,
) (models.CaseListPage, error) {
	if !filters.StartDate.IsZero() && !filters.EndDate.IsZero() &&
		filters.StartDate.After(filters.EndDate) {
		return models.CaseListPage{}, fmt.Errorf("start date must be before end date: %w", models.BadParameterError)
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
				StartDate:         filters.StartDate,
				EndDate:           filters.EndDate,
				Statuses:          filters.Statuses,
				OrganizationId:    organizationId,
				Name:              filters.Name,
				IncludeSnoozed:    filters.IncludeSnoozed,
				ExcludeAssigned:   filters.ExcludeAssigned,
				AssigneeId:        filters.AssigneeId,
				UseLinearOrdering: filters.UseLinearOrdering,
				TagId:             filters.TagId,
				Qualification:     filters.Qualification,
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
				if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
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

			return models.CaseListPage{
				Cases:       cases,
				HasNextPage: hasNextPage,
			}, nil
		},
	)
}

func (usecase *CaseUseCase) GetCasesReferents(ctx context.Context, caseIds []string) (map[string]models.CaseReferents, error) {
	referents, err := usecase.repository.GetCaseReferents(ctx,
		usecase.executorFactory.NewExecutor(), caseIds)
	if err != nil {
		return nil, err
	}

	referentMap := make(map[string]models.CaseReferents, len(referents))

	for _, ref := range referents {
		referentMap[ref.Id] = ref
	}

	return referentMap, nil
}

func (usecase *CaseUseCase) GetCaseComments(ctx context.Context, caseId string,
	paging models.PaginationAndSorting,
) (models.Paginated[models.CaseEvent], error) {
	c, err := usecase.GetCase(ctx, caseId)
	if err != nil {
		return models.Paginated[models.CaseEvent]{},
			errors.Wrap(err, "could not retrieve requested case")
	}

	inboxes, err := usecase.getAvailableInboxIds(ctx, usecase.executorFactory.NewExecutor(), c.OrganizationId)
	if err != nil {
		return models.Paginated[models.CaseEvent]{},
			errors.Wrap(err, "could not retrieve available inboxes")
	}

	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), inboxes); err != nil {
		return models.Paginated[models.CaseEvent]{}, err
	}

	pagingPlusOne := paging
	pagingPlusOne.Limit += 1

	comments, err := usecase.repository.ListCaseEventsOfTypes(ctx,
		usecase.executorFactory.NewExecutor(), caseId, []models.CaseEventType{
			models.CaseCommentAdded,
		}, pagingPlusOne)
	if err != nil {
		return models.Paginated[models.CaseEvent]{},
			errors.Wrap(err, "could not list comment case events")
	}

	return models.Paginated[models.CaseEvent]{
		Items:       comments[:min(len(comments), paging.Limit)],
		HasNextPage: len(comments) > paging.Limit,
	}, nil
}

func (usecase *CaseUseCase) GetCaseFiles(ctx context.Context, caseId string) ([]models.CaseFile, error) {
	c, err := usecase.GetCase(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve requested case")
	}

	inboxes, err := usecase.getAvailableInboxIds(ctx, usecase.executorFactory.NewExecutor(), c.OrganizationId)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve available inboxes")
	}

	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), inboxes); err != nil {
		return nil, err
	}

	caseFiles, err := usecase.repository.GetCasesFileByCaseId(ctx,
		usecase.executorFactory.NewExecutor(), caseId)
	if err != nil {
		return nil, err
	}

	return caseFiles, nil
}

func (usecase *CaseUseCase) getAvailableInboxIds(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID) ([]uuid.UUID, error) {
	inboxes, err := usecase.inboxReader.ListInboxes(ctx, exec, organizationId, false)
	if err != nil {
		return []uuid.UUID{}, errors.Wrap(err, "failed to list available inboxes in usecase")
	}
	availableInboxIds := make([]uuid.UUID, len(inboxes))
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
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return models.Case{}, err
	}

	return c, nil
}

func (usecase *CaseUseCase) GetEntityRelatedCases(ctx context.Context, objectType, objectId string) ([]models.Case, error) {
	orgId := usecase.enforceSecurity.OrgId()
	exec := usecase.executorFactory.NewExecutor()

	cases, err := usecase.repository.GetCasesRelatedToObject(ctx, exec, orgId, objectType, objectId)
	if err != nil {
		return nil, err
	}

	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, orgId)
	if err != nil {
		return nil, err
	}

	allowedCases := make([]models.Case, 0, len(cases))

	for _, c := range cases {
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err == nil {
			allowedCases = append(allowedCases, c)
		}
	}

	return allowedCases, nil
}

func (usecase *CaseUseCase) ListCaseDecisions(ctx context.Context, req models.CaseDecisionsRequest) ([]models.DecisionWithRulesAndScreeningsBaseInfo, bool, error) {
	_, err := usecase.GetCase(ctx, req.CaseId)
	if err != nil {
		return nil, false, err
	}

	decisions, hasMore, err := usecase.decisionRepository.DecisionsByCaseIdFromCursor(ctx,
		usecase.executorFactory.NewExecutor(), req)
	if err != nil {
		return nil, false, err
	}

	return decisions, hasMore, nil
}

func (usecase *CaseUseCase) CreateCase(
	ctx context.Context,
	tx repositories.Transaction,
	userId string,
	createCaseAttributes models.CreateCaseAttributes,
	fromEndUser bool,
) (models.Case, error) {
	if err := usecase.validateDecisions(ctx, tx, createCaseAttributes.OrganizationId,
		createCaseAttributes.DecisionIds); err != nil {
		return models.Case{}, err
	}
	if err := usecase.validateContinuousScreenings(
		ctx,
		tx,
		createCaseAttributes.ContinuousScreeningIds,
	); err != nil {
		return models.Case{}, err
	}

	newCaseId := uuid.NewString()
	err := usecase.repository.CreateCase(ctx, tx, createCaseAttributes, newCaseId)
	if err != nil {
		return models.Case{}, err
	}

	if err := usecase.triggerAutoAssignment(ctx, tx, createCaseAttributes.OrganizationId,
		createCaseAttributes.InboxId); err != nil {
		return models.Case{}, errors.Wrap(err, "could not trigger auto-assignment")
	}

	if fromEndUser {
		if _, err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			OrgId:     createCaseAttributes.OrganizationId,
			CaseId:    newCaseId,
			UserId:    &userId,
			EventType: models.CaseCreated,
		}); err != nil {
			return models.Case{}, err
		}
		if _, err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			OrgId:     createCaseAttributes.OrganizationId,
			CaseId:    newCaseId,
			UserId:    &userId,
			EventType: models.CaseAssigned,
			NewValue:  &userId,
		}); err != nil {
			return models.Case{}, err
		}
		if err := usecase.createCaseContributorIfNotExist(ctx, tx, newCaseId, userId); err != nil {
			return models.Case{}, err
		}

	} else {
		if _, err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			OrgId:     createCaseAttributes.OrganizationId,
			CaseId:    newCaseId,
			EventType: models.CaseCreated,
		}); err != nil {
			return models.Case{}, err
		}
	}

	err = usecase.UpdateDecisionsWithEvents(ctx, tx,
		createCaseAttributes.OrganizationId, newCaseId, userId, createCaseAttributes.DecisionIds)
	if err != nil {
		return models.Case{}, err
	}
	err = usecase.updateContinuousScreeningsWithEvents(
		ctx,
		tx,
		createCaseAttributes.OrganizationId,
		newCaseId,
		userId,
		createCaseAttributes.ContinuousScreeningIds,
	)
	if err != nil {
		return models.Case{}, err
	}

	return usecase.getCaseWithDetails(ctx, tx, newCaseId)
}

func (usecase *CaseUseCase) CreateCaseAsUser(
	ctx context.Context,
	organizationId uuid.UUID,
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
				EventContent:   models.NewWebhookEventCaseCreatedManually(newCase),
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

func (usecase *CaseUseCase) CreateCaseAsApiClient(
	ctx context.Context,
	orgId uuid.UUID,
	createCaseAttributes models.CreateCaseAttributes,
) (models.Case, error) {
	webhookEventId := uuid.NewString()
	c, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory,
		func(tx repositories.Transaction) (models.Case, error) {
			availableInboxIds, err := usecase.getAvailableInboxIds(ctx, tx, orgId)
			if err != nil {
				return models.Case{}, err
			}
			if err := usecase.enforceSecurity.CreateCase(createCaseAttributes, availableInboxIds); err != nil {
				return models.Case{}, err
			}

			newCase, err := usecase.CreateCase(ctx, tx, "", createCaseAttributes, false)
			if err != nil {
				return models.Case{}, err
			}

			err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
				Id:             webhookEventId,
				OrganizationId: newCase.OrganizationId,
				EventContent:   models.NewWebhookEventCaseCreatedManually(newCase),
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
	updateDone := false

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
			tx,
			c.OrganizationId)
		if err != nil {
			return models.Case{}, err
		}
		if updateCaseAttributes.InboxId != nil {
			// access check on the case's new requested inbox
			if _, err := usecase.inboxReader.GetInboxById(ctx, tx,
				*updateCaseAttributes.InboxId); err != nil {
				return models.Case{}, errors.WithDetail(errors.Wrap(err,
					fmt.Sprintf("User does not have access the new inbox %s", updateCaseAttributes.InboxId)),
					"assigned user does not have access to the target inbox")
			}
		}
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
			return models.Case{}, err
		}

		if c.Status == models.CasePending && (updateCaseAttributes.Status == "" ||
			updateCaseAttributes.Status == models.CasePending) {
			updateCaseAttributes.Status = models.CaseInvestigating
		}

		if updateCaseAttributes.Status != "" && !c.Status.CanTransition(updateCaseAttributes.Status) {
			return c, errors.Wrap(models.BadParameterError,
				fmt.Sprintf("invalid case status transition from %s to %s", c.Status, updateCaseAttributes.Status))
		}

		if updateCaseAttributes.Outcome != "" && !slices.Contains(models.ValidCaseOutcomes, updateCaseAttributes.Outcome) {
			return c, errors.Wrap(models.BadParameterError,
				fmt.Sprintf("invalid case outcome '%s'", updateCaseAttributes.Outcome))
		}

		err = usecase.repository.UpdateCase(ctx, tx, updateCaseAttributes)
		if err != nil {
			return models.Case{}, err
		}

		switch updateCaseAttributes.Status {
		case "":
			if err := usecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
				return models.Case{}, err
			}
		default:
			if err := usecase.performCaseActionSideEffectsWithoutStatusChange(ctx, tx, c); err != nil {
				return models.Case{}, err
			}

			if updateCaseAttributes.Status == models.CaseClosed {
				if err := usecase.triggerAutoAssignment(ctx, tx,
					c.OrganizationId, c.InboxId); err != nil {
					return models.Case{}, errors.Wrap(err, "could not trigger auto-assignment")
				}
			}
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

		updateDone = true
		return updatedCase, nil
	})
	if err != nil {
		return models.Case{}, err
	}

	if updateDone {
		usecase.webhookEventsUsecase.SendWebhookEventAsync(ctx, webhookEventId)
		trackCaseUpdatedEvents(ctx, updatedCase.Id, updateCaseAttributes)
	}

	return updatedCase, nil
}

func (usecase *CaseUseCase) Snooze(ctx context.Context, req models.CaseSnoozeRequest) error {
	c, err := usecase.repository.GetCaseById(ctx, usecase.executorFactory.NewExecutor(), req.CaseId)
	if err != nil {
		return err
	}

	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), []uuid.UUID{c.InboxId}); err != nil {
		return err
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.repository.SnoozeCase(ctx, tx, req); err != nil {
			return err
		}

		var previousSnooze *string

		if c.IsSnoozed() {
			previousSnooze = utils.Ptr(c.SnoozedUntil.Format(time.RFC3339))
		}

		// Case side effects should be called before snoozing, since it removes the boost.
		if err := usecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
			return err
		}
		if err := usecase.repository.BoostCase(ctx, tx, req.CaseId, models.BoostUnsnoozed); err != nil {
			return err
		}

		event := models.CreateCaseEventAttributes{
			OrgId:         c.OrganizationId,
			UserId:        utils.Ptr(string(req.UserId)),
			CaseId:        req.CaseId,
			EventType:     models.CaseSnoozed,
			NewValue:      utils.Ptr(req.Until.Format(time.RFC3339)),
			PreviousValue: previousSnooze,
		}

		if _, err = usecase.repository.CreateCaseEvent(ctx, tx, event); err != nil {
			return err
		}

		return nil
	})
}

func (usecase *CaseUseCase) Unsnooze(ctx context.Context, req models.CaseSnoozeRequest) error {
	c, err := usecase.repository.GetCaseById(ctx, usecase.executorFactory.NewExecutor(), req.CaseId)
	if err != nil {
		return err
	}

	if !c.IsSnoozed() {
		return nil
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), []uuid.UUID{c.InboxId}); err != nil {
			return err
		}

		// Case side effects should be called before unsnoozing, since it removes the boost.
		if err := usecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
			return err
		}
		if err = usecase.repository.UnsnoozeCase(ctx, tx, req.CaseId); err != nil {
			return err
		}

		event := models.CreateCaseEventAttributes{
			OrgId:         c.OrganizationId,
			UserId:        utils.Ptr(string(req.UserId)),
			CaseId:        req.CaseId,
			EventType:     models.CaseUnsnoozed,
			PreviousValue: utils.Ptr(c.SnoozedUntil.Format(time.RFC3339)),
		}

		if _, err = usecase.repository.CreateCaseEvent(ctx, tx, event); err != nil {
			return err
		}

		return err
	})
}

func (usecase *CaseUseCase) SelfAssignOnAction(ctx context.Context, tx repositories.Executor, orgId uuid.UUID, caseId, userId string) error {
	if err := usecase.repository.AssignCase(ctx, tx, caseId, utils.Ptr(models.UserId(userId))); err != nil {
		return err
	}

	if _, err := usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
		OrgId:     orgId,
		CaseId:    caseId,
		UserId:    &userId,
		EventType: models.CaseAssigned,
		NewValue:  &userId,
	}); err != nil {
		return err
	}

	return nil
}

func (usecase *CaseUseCase) AssignCase(ctx context.Context, req models.CaseAssignementRequest) error {
	c, err := usecase.repository.GetCaseById(ctx, usecase.executorFactory.NewExecutor(), req.CaseId)
	if err != nil {
		return err
	}

	if c.Status.IsFinalized() {
		return errors.Wrap(models.BadParameterError, "cannot reassign a closed case")
	}

	if req.AssigneeId == nil {
		return errors.Wrap(models.BadParameterError, "cannot assign to a null user")
	}

	if c.AssignedTo != nil && *c.AssignedTo == *req.AssigneeId {
		return nil
	}

	availableInboxIds, err := usecase.getAvailableInboxIds(ctx,
		usecase.executorFactory.NewExecutor(), c.OrganizationId)
	if err != nil {
		return err
	}
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return err
	}

	assignee, err := usecase.repository.UserById(ctx, usecase.executorFactory.NewExecutor(), string(*req.AssigneeId))
	if err != nil {
		return errors.Wrap(err, "target user for assignment not found")
	}

	if err := security.EnforceSecurityCaseForUser(assignee).ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return errors.Wrap(err, "target user lacks case permissions for assignment")
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.repository.AssignCase(ctx, tx, req.CaseId, req.AssigneeId); err != nil {
			return err
		}

		if err := usecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
			return err
		}

		var userId *string

		if req.UserId != "" {
			userId = utils.Ptr(string(req.UserId))
		}

		if _, err := usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			OrgId:         c.OrganizationId,
			CaseId:        c.Id,
			UserId:        userId,
			EventType:     models.CaseAssigned,
			NewValue:      (*string)(req.AssigneeId),
			PreviousValue: (*string)(c.AssignedTo),
		}); err != nil {
			return err
		}

		return nil
	})
}

func (usecase *CaseUseCase) UnassignCase(ctx context.Context, req models.CaseAssignementRequest) error {
	c, err := usecase.repository.GetCaseById(ctx, usecase.executorFactory.NewExecutor(), req.CaseId)
	if err != nil {
		return err
	}

	if c.Status.IsFinalized() {
		return errors.Wrap(models.BadParameterError, "cannot reassign a closed case")
	}

	if c.AssignedTo == nil {
		return nil
	}

	availableInboxIds, err := usecase.getAvailableInboxIds(ctx,
		usecase.executorFactory.NewExecutor(), c.OrganizationId)
	if err != nil {
		return err
	}
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return err
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.repository.UnassignCase(ctx, tx, req.CaseId); err != nil {
			return err
		}

		var userId *string

		if req.UserId != "" {
			userId = utils.Ptr(string(req.UserId))
		}

		if _, err := usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			OrgId:         c.OrganizationId,
			CaseId:        c.Id,
			UserId:        userId,
			EventType:     models.CaseAssigned,
			NewValue:      nil,
			PreviousValue: (*string)(c.AssignedTo),
		}); err != nil {
			return err
		}

		return nil
	})
}

func isIdenticalCaseUpdate(updateCaseAttributes models.UpdateCaseAttributes, c models.Case) bool {
	return (updateCaseAttributes.Name == "" || updateCaseAttributes.Name == c.Name) &&
		(updateCaseAttributes.Status == "" || updateCaseAttributes.Status == c.Status) &&
		(updateCaseAttributes.InboxId == nil || *updateCaseAttributes.InboxId == c.InboxId) &&
		(updateCaseAttributes.Outcome == "" || updateCaseAttributes.Outcome == c.Outcome)
}

func (usecase *CaseUseCase) updateCaseCreateEvents(ctx context.Context, exec repositories.Executor,
	updateCaseAttributes models.UpdateCaseAttributes, oldCase models.Case, userId string,
) error {
	var err error
	if updateCaseAttributes.Name != "" && updateCaseAttributes.Name != oldCase.Name {
		_, err = usecase.repository.CreateCaseEvent(ctx, exec, models.CreateCaseEventAttributes{
			OrgId:         oldCase.OrganizationId,
			CaseId:        updateCaseAttributes.Id,
			UserId:        &userId,
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
		_, err = usecase.repository.CreateCaseEvent(ctx, exec, models.CreateCaseEventAttributes{
			OrgId:         oldCase.OrganizationId,
			CaseId:        updateCaseAttributes.Id,
			UserId:        &userId,
			EventType:     models.CaseStatusUpdated,
			NewValue:      &newStatus,
			PreviousValue: (*string)(&oldCase.Status),
		})
		if err != nil {
			return err
		}
	}

	if updateCaseAttributes.Outcome != "" && updateCaseAttributes.Outcome != oldCase.Outcome {
		newOutcome := string(updateCaseAttributes.Outcome)
		_, err = usecase.repository.CreateCaseEvent(ctx, exec, models.CreateCaseEventAttributes{
			OrgId:         oldCase.OrganizationId,
			CaseId:        updateCaseAttributes.Id,
			UserId:        &userId,
			EventType:     models.CaseOutcomeUpdated,
			NewValue:      &newOutcome,
			PreviousValue: (*string)(&oldCase.Outcome),
		})
		if err != nil {
			return err
		}
	}

	if updateCaseAttributes.InboxId != nil && *updateCaseAttributes.InboxId != oldCase.InboxId {
		_, err = usecase.repository.CreateCaseEvent(ctx, exec, models.CreateCaseEventAttributes{
			OrgId:         oldCase.OrganizationId,
			CaseId:        updateCaseAttributes.Id,
			UserId:        &userId,
			EventType:     models.CaseInboxChanged,
			NewValue:      utils.Ptr(updateCaseAttributes.InboxId.String()),
			PreviousValue: utils.Ptr(oldCase.InboxId.String()),
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
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
			return models.Case{}, err
		}
		if c.Type != models.CaseTypeDecision {
			return models.Case{}, errors.Wrap(
				models.BadParameterError,
				"can not add decisions to this case type",
			)
		}
		if err := usecase.validateDecisions(ctx, tx, c.OrganizationId, decisionIds); err != nil {
			return models.Case{}, err
		}

		err = usecase.UpdateDecisionsWithEvents(ctx, tx, c.OrganizationId, caseId, userId, decisionIds)
		if err != nil {
			return models.Case{}, err
		}

		if err := usecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
			return models.Case{}, err
		}

		if len(c.Decisions) > 0 {
			if err := usecase.repository.BoostCase(ctx, tx, caseId, models.BoostNewDecision); err != nil {
				return models.Case{}, err
			}
		}

		updatedCase, err := usecase.getCaseWithDetails(ctx, tx, caseId)
		if err != nil {
			return models.Case{}, err
		}

		err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: updatedCase.OrganizationId,
			EventContent:   models.NewWebhookEventCaseDecisionsUpdated(updatedCase),
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
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
			return models.Case{}, err
		}

		caseEvent, err := usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			OrgId:          c.OrganizationId,
			CaseId:         caseCommentAttributes.Id,
			UserId:         &userId,
			EventType:      models.CaseCommentAdded,
			AdditionalNote: &caseCommentAttributes.Comment,
		})
		if err != nil {
			return models.Case{}, err
		}

		if err := usecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
			return models.Case{}, err
		}

		updatedCase, err := usecase.getCaseWithDetails(ctx, tx, caseCommentAttributes.Id)
		if err != nil {
			return models.Case{}, err
		}

		err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: updatedCase.OrganizationId,
			EventContent:   models.NewWebhookEventCaseCommentCreated(updatedCase, caseEvent),
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
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
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
		_, err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			OrgId:         c.OrganizationId,
			CaseId:        caseTagAttributes.CaseId,
			UserId:        &userId,
			EventType:     models.CaseTagsUpdated,
			PreviousValue: &previousValue,
			NewValue:      &newValue,
		})
		if err != nil {
			return models.Case{}, err
		}

		updatedCase, err := usecase.getCaseWithDetails(ctx, tx, caseTagAttributes.CaseId)
		if err != nil {
			return models.Case{}, err
		}

		if err := usecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
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

	if tag.Target != models.TagTargetCase {
		return errors.Wrap(models.BadParameterError, "provided tag is not targeting cases")
	}
	if tag.DeletedAt != nil {
		return fmt.Errorf("tag %s is deleted %w", tag.Id, models.BadParameterError)
	}

	if err = usecase.repository.CreateCaseTag(ctx, exec, caseId, tagId); err != nil {
		return err
	}

	return nil
}

func (usecase *CaseUseCase) getCaseWithDetails(ctx context.Context, exec repositories.Executor, caseId string) (models.Case, error) {
	c, err := usecase.repository.GetCaseById(ctx, exec, caseId)
	if err != nil {
		return models.Case{}, err
	}

	switch c.Type {
	case models.CaseTypeDecision:
		decisions, err := usecase.decisionRepository.DecisionsByCaseId(ctx, exec, c.OrganizationId, caseId)
		if err != nil {
			return models.Case{}, err
		}
		c.Decisions = decisions

	case models.CaseTypeContinuousScreening:
		continuousScreeningsWithMatches, err :=
			usecase.repository.ListContinuousScreeningsWithMatchesByCaseId(ctx, exec, caseId)
		if err != nil {
			return models.Case{}, err
		}

		c.ContinuousScreenings = continuousScreeningsWithMatches

	default:
		return models.Case{}, errors.Errorf("case type %s is not supported", c.Type)
	}

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

func (usecase *CaseUseCase) validateDecisions(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, decisionIds []string) error {
	if len(decisionIds) == 0 {
		return nil
	}
	decisions, err := usecase.decisionRepository.DecisionsById(ctx, exec, decisionIds)
	if err != nil {
		return err
	}

	for _, decision := range decisions {
		if decision.OrganizationId != orgId {
			return errors.WithDetail(errors.Wrap(models.ForbiddenError,
				"provided decision does not belong to the organization"),
				"some of the provided decisions do not exist")
		}

		if decision.Case != nil && decision.Case.Id != "" {
			return errors.WithDetailf(errors.Wrapf(models.BadParameterError,
				"decision %s already belongs to a case %s",
				decision.DecisionId, (*decision.Case).Id),
				"provided decision '%s' is already assigned to a case", decision.DecisionId)
		}
	}

	if len(decisionIds) != len(decisions) {
		return errors.WithDetail(errors.Wrap(models.NotFoundError, "unknown decision"),
			"some of the provided decisions do not exist")
	}

	return nil
}

func (usecase *CaseUseCase) validateContinuousScreenings(
	ctx context.Context,
	exec repositories.Executor,
	continuousScreeningIds []uuid.UUID,
) error {
	if len(continuousScreeningIds) == 0 {
		return nil
	}

	continuousScreenings, err := usecase.repository.ListContinuousScreeningsByIds(ctx, exec, continuousScreeningIds)
	if err != nil {
		return err
	}
	for _, continuousScreening := range continuousScreenings {
		if continuousScreening.CaseId != nil {
			return fmt.Errorf("continuous screening %s already belongs to a case %s %w",
				continuousScreening.Id.String(), continuousScreening.CaseId.String(), models.BadParameterError)
		}
	}

	return nil
}

func (usecase *CaseUseCase) UpdateDecisionsWithEvents(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
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
				OrgId:        orgId,
				CaseId:       caseId,
				UserId:       &userId,
				EventType:    models.DecisionAdded,
				ResourceId:   &decisionId,
				ResourceType: &resourceType,
			}
		}
		if _, err := usecase.repository.BatchCreateCaseEvents(ctx, exec,
			createCaseEventAttributes); err != nil {
			return err
		}
	}
	return nil
}

func (usecase *CaseUseCase) updateContinuousScreeningsWithEvents(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	caseId string,
	userId string,
	continuousScreeningIdsToAdd []uuid.UUID,
) error {
	if len(continuousScreeningIdsToAdd) > 0 {
		if err := usecase.repository.UpdateContinuousScreeningsCaseId(
			ctx,
			exec,
			continuousScreeningIdsToAdd,
			caseId,
		); err != nil {
			return err
		}

		err := usecase.repository.UnsnoozeCase(ctx, exec, caseId)
		if err != nil {
			return err
		}

		createCaseEventAttributes := make([]models.CreateCaseEventAttributes, len(continuousScreeningIdsToAdd))
		resourceType := models.ContinuousScreeningResourceType
		for i, continuousScreeningId := range continuousScreeningIdsToAdd {
			createCaseEventAttributes[i] = models.CreateCaseEventAttributes{
				OrgId:        orgId,
				CaseId:       caseId,
				UserId:       &userId,
				EventType:    models.ContinuousScreeningAdded,
				ResourceId:   utils.Ptr(continuousScreeningId.String()),
				ResourceType: &resourceType,
			}
		}
		if _, err := usecase.repository.BatchCreateCaseEvents(ctx, exec,
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

func (usecase *CaseUseCase) CreateCaseFiles(ctx context.Context, input models.CreateCaseFilesInput) (models.Case, []models.CaseFile, error) {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	creds, found := utils.CredentialsFromCtx(ctx)
	if !found {
		return models.Case{}, nil, errors.New("no credentials in context")
	}
	userId := string(creds.ActorIdentity.UserId)

	for _, fileHeader := range input.Files {
		if err := validateFileType(fileHeader); err != nil {
			return models.Case{}, nil, err
		}
	}

	// permissions check
	c, err := usecase.repository.GetCaseById(ctx, exec, input.CaseId)
	if err != nil {
		return models.Case{}, nil, err
	}
	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, c.OrganizationId)
	if err != nil {
		return models.Case{}, nil, err
	}
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return models.Case{}, nil, err
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
		return models.Case{}, nil, err
	}

	caseFiles := make([]models.CaseFile, len(input.Files))

	webhookEventId := uuid.NewString()
	err = usecase.transactionFactory.Transaction(ctx, func(
		tx repositories.Transaction,
	) error {
		for idx, uploadedFile := range uploadedFilesMetadata {
			newCaseFileId := uuid.NewString()
			caseFile, err := usecase.repository.CreateDbCaseFile(
				ctx,
				tx,
				models.CreateDbCaseFileInput{
					Id:            newCaseFileId,
					BucketName:    usecase.caseManagerBucketUrl,
					CaseId:        input.CaseId,
					FileName:      uploadedFile.fileName,
					FileReference: uploadedFile.fileReference,
				},
			)
			if err != nil {
				return err
			}

			caseFiles[idx] = caseFile

			resourceType := models.CaseFileResourceType
			_, err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
				OrgId:          creds.OrganizationId,
				CaseId:         input.CaseId,
				UserId:         &userId,
				EventType:      models.CaseFileAdded,
				ResourceType:   &resourceType,
				ResourceId:     &newCaseFileId,
				AdditionalNote: &uploadedFile.fileName,
			})
			if err != nil {
				return err
			}
		}

		if err := usecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
			return err
		}

		// Create a single webhook event for the case
		err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: creds.OrganizationId,
			EventContent:   models.NewWebhookEventCaseFileCreated(c, caseFiles),
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return models.Case{}, nil, err
	}

	usecase.webhookEventsUsecase.SendWebhookEventAsync(ctx, webhookEventId)
	// Dispatch one event per file uploaded, so it's easier to do aggregation in the analytics
	for range uploadedFilesMetadata {
		tracking.TrackEvent(ctx, models.AnalyticsCaseFileCreated, map[string]interface{}{
			"case_id": input.CaseId,
		})
	}

	caseDetails, err := usecase.getCaseWithDetails(ctx, exec, input.CaseId)
	if err != nil {
		return models.Case{}, nil, err
	}

	return caseDetails, caseFiles, nil
}

func (usecase *CaseUseCase) AttachAnnotation(ctx context.Context, tx repositories.Transaction,
	annotationId string, annotationReq models.CreateEntityAnnotationRequest,
) error {
	if annotationReq.CaseId == nil {
		return errors.New("tried to attach annotation to a case without a case ID")
	}

	inboxes, err := usecase.getAvailableInboxIds(ctx, tx, annotationReq.OrgId)
	if err != nil {
		return err
	}
	c, err := usecase.GetCase(ctx, *annotationReq.CaseId)
	if err != nil {
		return err
	}

	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), inboxes); err != nil {
		return errors.Wrap(models.ForbiddenError, err.Error())
	}

	_, err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
		OrgId:          annotationReq.OrgId,
		CaseId:         *annotationReq.CaseId,
		UserId:         (*string)(annotationReq.AnnotatedBy),
		EventType:      models.CaseEntityAnnotated,
		ResourceType:   utils.Ptr(models.AnnotationResourceType),
		ResourceId:     &annotationId,
		AdditionalNote: utils.Ptr(annotationReq.AnnotationType.String()),
	})

	return err
}

func (usecase *CaseUseCase) AttachAnnotationFiles(ctx context.Context, tx repositories.Transaction,
	annotationId string, annotationReq models.CreateEntityAnnotationRequest, files []models.EntityAnnotationFilePayloadFile,
) error {
	if annotationReq.CaseId == nil {
		return errors.New("tried to attach file annotation to a case without a case ID")
	}

	inboxes, err := usecase.getAvailableInboxIds(ctx, tx, annotationReq.OrgId)
	if err != nil {
		return err
	}
	c, err := usecase.GetCase(ctx, *annotationReq.CaseId)
	if err != nil {
		return err
	}

	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), inboxes); err != nil {
		return errors.Wrap(models.ForbiddenError, err.Error())
	}

	for _, file := range files {
		newFileUuid := uuid.NewString()

		_, err := usecase.repository.CreateDbCaseFile(ctx, tx, models.CreateDbCaseFileInput{
			Id:            newFileUuid,
			BucketName:    usecase.caseManagerBucketUrl,
			CaseId:        *annotationReq.CaseId,
			FileName:      file.Filename,
			FileReference: file.Key,
		})
		if err != nil {
			return err
		}
	}

	_, err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
		OrgId:          annotationReq.OrgId,
		CaseId:         *annotationReq.CaseId,
		UserId:         (*string)(annotationReq.AnnotatedBy),
		EventType:      models.CaseEntityAnnotated,
		ResourceType:   utils.Ptr(models.AnnotationResourceType),
		ResourceId:     &annotationId,
		AdditionalNote: utils.Ptr(annotationReq.AnnotationType.String()),
	})

	return err
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
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
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
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return err
	}

	if err := usecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
		return err
	}

	resourceType := models.RuleSnoozeResourceType
	_, err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
		OrgId:          c.OrganizationId,
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
		EventContent: models.NewWebhookEventCaseCommentCreated(updatedCase, models.CaseEvent{
			UserId:         null.NewString(*input.UserId, true),
			AdditionalNote: input.Comment,
		}),
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

	if err := usecase.enforceSecurityDecision.ReadDecision(decision); err != nil {
		return models.Case{}, errors.Wrapf(models.ForbiddenError,
			"not allowed to access decision %s", input.DecisionId)
	}

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
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return models.Case{}, err
	}
	if c.Type != models.CaseTypeDecision {
		return models.Case{}, errors.Wrap(
			models.BadParameterError,
			"can not review decisions on this case type",
		)
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
			decisionsAfterReview, err := usecase.decisionRepository.DecisionsById(ctx, tx, []string{input.DecisionId})
			if err != nil {
				return models.Case{}, err
			}
			if len(decisionsAfterReview) == 0 {
				return models.Case{}, errors.Wrapf(models.NotFoundError,
					"decision %s not found after review, should not happen", input.DecisionId)
			}

			resourceType := models.DecisionResourceType
			_, err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
				OrgId:          c.OrganizationId,
				CaseId:         caseId,
				UserId:         &input.UserId,
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

			if err := usecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
				return models.Case{}, err
			}

			err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
				Id:             webhookEventId,
				OrganizationId: c.OrganizationId,
				EventContent:   models.NewWebhookEventDecisionReviewed(c, decisionsAfterReview[0]),
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

func (usecase *CaseUseCase) GetRelatedCasesByPivotValue(ctx context.Context, orgId uuid.UUID, pivotValue string) ([]models.Case, error) {
	exec := usecase.executorFactory.NewExecutor()

	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, orgId)
	if err != nil {
		return nil, err
	}

	cases, err := usecase.repository.GetCasesWithPivotValue(ctx, exec, orgId, pivotValue)
	if err != nil {
		return nil, err
	}

	allowedCases := make([]models.Case, 0, len(cases))

	for _, c := range cases {
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err == nil {
			allowedCases = append(allowedCases, c)
		}
	}

	return allowedCases, nil
}

func (usecase *CaseUseCase) GetRelatedContinuousScreeningCasesByObjectAttr(
	ctx context.Context, orgId uuid.UUID, objectType, objectId string,
) ([]models.Case, error) {
	exec := usecase.executorFactory.NewExecutor()

	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, orgId)
	if err != nil {
		return nil, err
	}

	cases, err := usecase.repository.GetContinuousScreeningCasesWithObjectAttr(ctx, exec, orgId, objectType, objectId)
	if err != nil {
		return nil, err
	}

	entityId := pure_utils.MarbleEntityIdBuilder(objectType, objectId)
	casesFromMatches, err := usecase.repository.GetContinuousScreeningCasesByEntityIdInMatches(ctx, exec, orgId, entityId)
	if err != nil {
		return nil, err
	}

	// We should have different cases from the two queries, we can combine them safely
	// No collision is expected between cases from different continuous screening types (Object*Triggered/DatasetUpdateTriggered)
	cases = append(cases, casesFromMatches...)

	// Sort by created_at descending
	slices.SortFunc(cases, func(a, b models.Case) int {
		if a.CreatedAt.After(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.Before(b.CreatedAt) {
			return 1
		}
		return 0
	})

	allowedCases := make([]models.Case, 0, len(cases))

	for _, c := range cases {
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err == nil {
			allowedCases = append(allowedCases, c)
		}
	}

	return allowedCases, nil
}

func (usecase *CaseUseCase) GetNextCaseId(ctx context.Context, orgId uuid.UUID, caseId string) (string, error) {
	exec := usecase.executorFactory.NewExecutor()

	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, orgId)
	if err != nil {
		return "", err
	}

	c, err := usecase.repository.GetCaseById(ctx, exec, caseId)
	if err != nil {
		return "", err
	}

	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return "", err
	}

	nextCaseId, err := usecase.repository.GetNextCase(ctx, exec, c)
	if err != nil {
		return "", err
	}

	return nextCaseId, nil
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

func (usecase *CaseUseCase) ReadCasePivotObjects(ctx context.Context, caseId string) ([]models.PivotObject, error) {
	exec := usecase.executorFactory.NewExecutor()
	c, err := usecase.repository.GetCaseMetadataById(ctx, exec, caseId)
	if err != nil {
		return nil, err
	}

	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, c.OrganizationId)
	if err != nil {
		return nil, err
	}
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
		return nil, err
	}

	pivotValues, err := usecase.repository.DecisionPivotValuesByCase(ctx, exec, caseId)
	if err != nil {
		return nil, err
	}

	return usecase.ingestedDataReader.ReadPivotObjectsFromValues(ctx, c.OrganizationId, pivotValues)
}

func (usecase *CaseUseCase) EscalateCase(ctx context.Context, caseId string) error {
	exec := usecase.executorFactory.NewExecutor()
	c, err := usecase.repository.GetCaseById(ctx, exec, caseId)
	if err != nil {
		return err
	}
	if c.Status == models.CaseClosed {
		return errors.WithDetail(errors.Wrap(models.UnprocessableEntityError,
			"case is already closed, cannot escalate"),
			"case is already closed, cannot escalate")
	}

	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, c.OrganizationId)
	if err != nil {
		return err
	}
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return err
	}

	sourceInbox, err := usecase.inboxReader.GetInboxById(ctx, exec, c.InboxId)
	if err != nil {
		return errors.Wrap(err, "could not read source inbox")
	}
	if sourceInbox.EscalationInboxId == nil {
		return errors.WithDetail(errors.Wrap(models.UnprocessableEntityError,
			"the source inbox does not have escalation configured"),
			"the source inbox does not have escalation configured")
	}

	// Not using the inboxReader here because we do not want to check for permission. A user
	// escalating a case will usually not have access to the target inbox.
	targetInbox, err := usecase.inboxReader.GetEscalationInboxMetadata(ctx, *sourceInbox.EscalationInboxId)
	if err != nil {
		return errors.Wrap(err, "could not read target inbox")
	}
	if targetInbox.Status != models.InboxStatusActive {
		return errors.WithDetail(errors.Wrap(models.UnprocessableEntityError,
			"target inbox is inactive"), "target inbox is inactive")
	}

	var userId *string
	if creds, ok := utils.CredentialsFromCtx(ctx); ok {
		userId = utils.Ptr(string(creds.ActorIdentity.UserId))
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		targetInboxIdStr := targetInbox.Id.String()
		sourceInboxIdStr := sourceInbox.Id.String()

		if err := usecase.repository.EscalateCase(ctx, tx, caseId, targetInboxIdStr); err != nil {
			return errors.Wrap(err, "could not escalate case")
		}

		event := models.CreateCaseEventAttributes{
			OrgId:         c.OrganizationId,
			CaseId:        caseId,
			UserId:        userId,
			EventType:     models.CaseEscalated,
			NewValue:      &targetInboxIdStr,
			PreviousValue: &sourceInboxIdStr,
		}

		if _, err := usecase.repository.CreateCaseEvent(ctx, tx, event); err != nil {
			return err
		}

		hasAiCaseReviewEnabled, err := usecase.aiAgentUsecase.HasAiCaseReviewEnabled(ctx, c.OrganizationId)
		if err != nil {
			return errors.Wrap(err, "error checking if AI case review is enabled")
		}
		if hasAiCaseReviewEnabled {
			// direct read through repository, because we may not have permission on this inbox in this situation.
			inbox, err := usecase.repository.GetInboxById(ctx, tx, targetInbox.Id)
			if err != nil {
				return errors.Wrap(err, "error getting inbox")
			}
			if inbox.CaseReviewOnEscalate {
				caseReviewId := uuid.Must(uuid.NewV7())
				caseIdUuid, err := uuid.Parse(c.Id)
				if err != nil {
					return errors.Wrap(err, "could not parse case id")
				}
				err = usecase.taskQueueRepository.EnqueueCaseReviewTask(ctx, tx,
					c.OrganizationId, caseIdUuid, caseReviewId)
				if err != nil {
					return errors.Wrap(err, "error enqueuing case review task")
				}
			}

		}

		return nil
	})
}

func (usecase *CaseUseCase) performCaseActionSideEffectsWithoutStatusChange(ctx context.Context, tx repositories.Transaction, c models.Case) error {
	userId := usecase.enforceSecurity.UserId()
	if userId != nil && c.AssignedTo == nil {
		if err := usecase.SelfAssignOnAction(ctx, tx, c.OrganizationId, c.Id, *userId); err != nil {
			return err
		}
	}

	if err := usecase.repository.UnboostCase(ctx, tx, c.Id); err != nil {
		return err
	}

	// ⚠️ This should be done after any updates to the case (within this function and any calling functions) to avoid
	// deadlocks, though deadlocks are also retried by the transaction factory for safety. This also means that the side
	// effects should be called after all the "main" updates ⚠️
	if userId != nil {
		err := usecase.createCaseContributorIfNotExist(ctx, tx, c.Id, *userId)
		if err != nil {
			return err
		}
	}

	return nil
}

func (usecase *CaseUseCase) PerformCaseActionSideEffects(ctx context.Context, tx repositories.Transaction, c models.Case) error {
	if c.Status == models.CasePending {
		update := models.UpdateCaseAttributes{Id: c.Id, Status: models.CaseInvestigating}

		if err := usecase.repository.UpdateCase(ctx, tx, update); err != nil {
			return err
		}
	}

	if err := usecase.performCaseActionSideEffectsWithoutStatusChange(ctx, tx, c); err != nil {
		return err
	}

	return nil
}

func (usecase *CaseUseCase) triggerAutoAssignment(ctx context.Context, tx repositories.Transaction, orgId uuid.UUID, inboxId uuid.UUID) error {
	features, err := usecase.featureAccessReader.GetOrganizationFeatureAccess(ctx, orgId, nil)
	if err != nil {
		return errors.Wrap(err, "could not check feature access")
	}

	if features.CaseAutoAssign.IsAllowed() {
		enabled, err := usecase.inboxReader.GetAutoAssignmentEnabled(ctx, inboxId)
		if err != nil {
			return errors.Wrap(err, "could not read inbox")
		}

		if enabled {
			if err := usecase.taskQueueRepository.EnqueueAutoAssignmentTask(ctx, tx, orgId, inboxId); err != nil {
				return errors.Wrap(err, "could not enqueue auto-assignment job")
			}
		}
	}

	return nil
}

func (usecase *CaseUseCase) MassUpdate(ctx context.Context, req dto.CaseMassUpdateDto) error {
	exec := usecase.executorFactory.NewExecutor()
	orgId := usecase.enforceSecurity.OrgId()
	userId := usecase.enforceSecurity.UserId()

	sourceCases := make(map[string]models.Case, len(req.CaseIds))
	events := make(map[string]models.CreateCaseEventAttributes, len(req.CaseIds))

	var newAssignee models.User

	cases, err := usecase.repository.GetMassCasesByIds(ctx, exec, req.CaseIds)
	if err != nil {
		return errors.Wrap(err, "could not retrieve requested cases for mass update")
	}

	if len(cases) != len(req.CaseIds) {
		return errors.New("some requested cases for mass update do not exist")
	}

	casesMap := pure_utils.MapSliceToMap(cases, func(c models.Case) (string, models.Case) {
		return c.Id, c
	})

	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec, orgId)
	if err != nil {
		return errors.Wrap(err, "could not retrieve available inboxes")
	}

	if req.Action == models.CaseMassUpdateAssign.String() {
		var err error

		newAssignee, err = usecase.repository.UserById(ctx, exec, req.Assign.AssigneeId.String())
		if err != nil {
			return errors.Wrap(err, "target user for assignment not found")
		}
	}

	// For all cases in the mass update, we need to check the current user can manage them.
	for _, caseId := range req.CaseIds {
		c, ok := casesMap[caseId.String()]
		if !ok {
			return errors.Newf("requested cases '%s' for mass update does not exist", caseId)
		}

		if err := usecase.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
			return err
		}

		// If we are trying to mass-assign, we need to check, for each case, that the target user can manage the case.
		if req.Action == models.CaseMassUpdateAssign.String() {
			if err := security.EnforceSecurityCaseForUser(newAssignee).ReadOrUpdateCase(
				c.GetMetadata(), availableInboxIds); err != nil {
				return errors.Wrap(err, "target user lacks case permissions for assignment")
			}
		}

		sourceCases[c.Id] = c
	}

	// When changing the cases' inboxes, the user needs to have access to the target inbox.
	if req.Action == models.CaseMassUpdateMoveToInbox.String() {
		if _, err := usecase.inboxReader.GetInboxById(ctx, exec, req.MoveToInbox.InboxId); err != nil {
			return errors.Wrap(err, fmt.Sprintf("user does not have access the new inbox %s", req.MoveToInbox.InboxId))
		}
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		switch models.CaseMassUpdateActionFromString(req.Action) {
		case models.CaseMassUpdateClose:
			updatedIds, err := usecase.repository.CaseMassChangeStatus(ctx, tx, req.CaseIds, models.CaseClosed)
			if err != nil {
				return errors.Wrap(err, "could not update case status in mass update")
			}

			for _, updatedId := range updatedIds {
				events[updatedId.String()] = models.CreateCaseEventAttributes{
					OrgId:         orgId,
					UserId:        userId,
					CaseId:        updatedId.String(),
					EventType:     models.CaseStatusUpdated,
					PreviousValue: utils.Ptr(string(sourceCases[updatedId.String()].Status)),
					NewValue:      utils.Ptr(string(models.CaseClosed)),
				}
			}

		case models.CaseMassUpdateReopen:
			updatedIds, err := usecase.repository.CaseMassChangeStatus(ctx, tx, req.CaseIds, models.CasePending)
			if err != nil {
				return errors.Wrap(err, "could not updaet case status in mass update")
			}

			for _, updatedId := range updatedIds {
				events[updatedId.String()] = models.CreateCaseEventAttributes{
					OrgId:         orgId,
					UserId:        userId,
					CaseId:        updatedId.String(),
					EventType:     models.CaseStatusUpdated,
					PreviousValue: utils.Ptr(string(sourceCases[updatedId.String()].Status)),
					NewValue:      utils.Ptr(string(models.CasePending)),
				}
			}

		case models.CaseMassUpdateAssign:
			updatedIds, err := usecase.repository.CaseMassAssign(ctx, tx, req.CaseIds, req.Assign.AssigneeId)
			if err != nil {
				return errors.Wrap(err, "could not assign cases in mass update")
			}

			for _, updatedId := range updatedIds {
				events[updatedId.String()] = models.CreateCaseEventAttributes{
					OrgId:         orgId,
					UserId:        userId,
					CaseId:        updatedId.String(),
					EventType:     models.CaseAssigned,
					PreviousValue: (*string)(sourceCases[updatedId.String()].AssignedTo),
					NewValue:      utils.Ptr(req.Assign.AssigneeId.String()),
				}
			}

		case models.CaseMassUpdateMoveToInbox:
			updatedIds, err := usecase.repository.CaseMassMoveToInbox(ctx, tx, req.CaseIds, req.MoveToInbox.InboxId)
			if err != nil {
				return errors.Wrap(err, "could not change case inbox in mass update")
			}

			for _, updatedId := range updatedIds {
				events[updatedId.String()] = models.CreateCaseEventAttributes{
					OrgId:         orgId,
					UserId:        userId,
					CaseId:        updatedId.String(),
					EventType:     models.CaseInboxChanged,
					PreviousValue: utils.Ptr(sourceCases[updatedId.String()].InboxId.String()),
					NewValue:      utils.Ptr(req.MoveToInbox.InboxId.String()),
				}
			}

		default:
			return errors.Newf("unknown case mass update action %s", req.Action)
		}

		if len(events) == 0 {
			return nil
		}

		// TODO: perform relevant side effects

		if _, err := usecase.repository.BatchCreateCaseEvents(ctx, tx,
			slices.Collect(maps.Values(events))); err != nil {
			return errors.Wrap(err, "could not create case events in mass update")
		}

		return nil
	})
}
