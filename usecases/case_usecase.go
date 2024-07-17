package usecases

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"slices"
	"strings"

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

type webhookEventsUsecase interface {
	CreateWebhookEvent(
		ctx context.Context,
		tx repositories.Executor,
		input models.WebhookEventCreate,
	) error
	SendWebhookEventAsync(ctx context.Context, webhookEventId string)
}

type CaseUseCase struct {
	enforceSecurity      security.EnforceSecurityCase
	repository           CaseUseCaseRepository
	decisionRepository   repositories.DecisionRepository
	inboxReader          inboxes.InboxReader
	gcsRepository        repositories.GcsRepository
	gcsCaseManagerBucket string
	transactionFactory   executor_factory.TransactionFactory
	executorFactory      executor_factory.ExecutorFactory
	webhookEventsUsecase webhookEventsUsecase
}

func (usecase *CaseUseCase) ListCases(
	ctx context.Context,
	organizationId string,
	pagination models.PaginationAndSorting,
	filters dto.CaseFilters,
) ([]models.CaseWithRank, error) {
	if !filters.StartDate.IsZero() && !filters.EndDate.IsZero() &&
		filters.StartDate.After(filters.EndDate) {
		return []models.CaseWithRank{}, fmt.Errorf("start date must be before end date: %w", models.BadParameterError)
	}
	statuses, err := models.ValidateCaseStatuses(filters.Statuses)
	if err != nil {
		return []models.CaseWithRank{}, err
	}

	if err := models.ValidatePagination(pagination); err != nil {
		return []models.CaseWithRank{}, err
	}

	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) ([]models.CaseWithRank, error) {
			availableInboxIds, err := usecase.getAvailableInboxIds(ctx, tx)
			if err != nil {
				return []models.CaseWithRank{}, err
			}
			if len(filters.InboxIds) > 0 {
				for _, inboxId := range filters.InboxIds {
					if !slices.Contains(availableInboxIds, inboxId) {
						return []models.CaseWithRank{}, errors.Wrap(
							models.ForbiddenError, fmt.Sprintf("inbox %s is not accessible", inboxId))
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

			cases, err := usecase.repository.ListOrganizationCases(ctx, tx, repoFilters, pagination)
			if err != nil {
				return []models.CaseWithRank{}, err
			}
			for _, c := range cases {
				if err := usecase.enforceSecurity.ReadOrUpdateCase(c.Case, availableInboxIds); err != nil {
					return []models.CaseWithRank{}, err
				}
			}
			return cases, nil
		},
	)
}

func (usecase *CaseUseCase) getAvailableInboxIds(ctx context.Context, exec repositories.Executor) ([]string, error) {
	inboxes, err := usecase.inboxReader.ListInboxes(ctx, exec, false)
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

	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec)
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
	tx repositories.Executor,
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
	userId string,
	createCaseAttributes models.CreateCaseAttributes,
) (models.Case, error) {
	exec := usecase.executorFactory.NewExecutor()
	c, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory,
		func(tx repositories.Executor) (models.Case, error) {
			// permission check on the inbox as end user
			availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec)
			if err != nil {
				return models.Case{}, err
			}
			if err := usecase.enforceSecurity.CreateCase(createCaseAttributes, availableInboxIds); err != nil {
				return models.Case{}, err
			}

			return usecase.CreateCase(ctx, tx, userId, createCaseAttributes, true)
		})

	tracking.TrackEvent(ctx, models.AnalyticsCaseCreated, map[string]interface{}{
		"case_id": c.Id,
	})

	return c, err
}

func (usecase *CaseUseCase) UpdateCase(ctx context.Context, userId string,
	updateCaseAttributes models.UpdateCaseAttributes,
) (models.Case, error) {
	webhookEventId := uuid.New().String()

	updatedCase, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Executor,
	) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(ctx, tx, updateCaseAttributes.Id)
		if err != nil {
			return models.Case{}, err
		}

		if isIdenticalCaseUpdate(updateCaseAttributes, c) {
			return usecase.getCaseWithDetails(ctx, tx, updateCaseAttributes.Id)
		}

		availableInboxIds, err := usecase.getAvailableInboxIds(ctx, usecase.executorFactory.NewExecutor())
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

		// TODO(webhook): integration test for webhooks, refactor when webhooks are fully implemented
		if updateCaseAttributes.Status != "" {
			err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
				Id:             webhookEventId,
				OrganizationId: updatedCase.OrganizationId,
				EventContent:   models.NewWebhookEventCaseStatusUpdated(updateCaseAttributes.Status),
			})
			if err != nil {
				return models.Case{}, err
			}
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
	updatedCase, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Executor,
	) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(ctx, tx, caseId)
		if err != nil {
			return models.Case{}, err
		}
		availableInboxIds, err := usecase.getAvailableInboxIds(ctx, tx)
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

		return usecase.getCaseWithDetails(ctx, tx, caseId)
	})
	if err != nil {
		return models.Case{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsDecisionsAdded, map[string]interface{}{
		"case_id": updatedCase.Id,
	})
	return updatedCase, nil
}

func (usecase *CaseUseCase) CreateCaseComment(ctx context.Context, userId string,
	caseCommentAttributes models.CreateCaseCommentAttributes,
) (models.Case, error) {
	updatedCase, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Executor,
	) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(ctx, tx, caseCommentAttributes.Id)
		if err != nil {
			return models.Case{}, err
		}

		availableInboxIds, err := usecase.getAvailableInboxIds(ctx, tx)
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
		return usecase.getCaseWithDetails(ctx, tx, caseCommentAttributes.Id)
	})
	if err != nil {
		return models.Case{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsCaseCommentCreated, map[string]interface{}{
		"case_id": updatedCase.Id,
	})
	return updatedCase, nil
}

func (usecase *CaseUseCase) CreateCaseTags(ctx context.Context, userId string,
	caseTagAttributes models.CreateCaseTagsAttributes,
) (models.Case, error) {
	updatedCase, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Executor,
	) (models.Case, error) {
		c, err := usecase.repository.GetCaseById(ctx, tx, caseTagAttributes.CaseId)
		if err != nil {
			return models.Case{}, err
		}

		availableInboxIds, err := usecase.getAvailableInboxIds(ctx, usecase.executorFactory.NewExecutor())
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

		return usecase.getCaseWithDetails(ctx, tx, caseTagAttributes.CaseId)
	})
	if err != nil {
		return models.Case{}, err
	}

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

func (usecase *CaseUseCase) CreateCaseFile(ctx context.Context, input models.CreateCaseFileInput) (models.Case, error) {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	creds, found := utils.CredentialsFromCtx(ctx)
	if !found {
		return models.Case{}, errors.New("no credentials in context")
	}
	userId := string(creds.ActorIdentity.UserId)

	if err := validateFileType(input.File); err != nil {
		return models.Case{}, err
	}

	// permissions check
	c, err := usecase.repository.GetCaseById(ctx, exec, input.CaseId)
	if err != nil {
		return models.Case{}, err
	}
	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec)
	if err != nil {
		return models.Case{}, err
	}
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
		return models.Case{}, err
	}

	newFileReference := fmt.Sprintf("%s/%s/%s", creds.OrganizationId, input.CaseId, uuid.NewString())
	writer := usecase.gcsRepository.OpenStream(ctx, usecase.gcsCaseManagerBucket, newFileReference)
	file, err := input.File.Open()
	if err != nil {
		return models.Case{}, errors.Wrap(models.BadParameterError, err.Error())
	}
	if _, err := io.Copy(writer, file); err != nil {
		return models.Case{}, err
	}
	if err := writer.Close(); err != nil {
		return models.Case{}, err
	}
	if err := usecase.gcsRepository.UpdateFileMetadata(
		ctx,
		usecase.gcsCaseManagerBucket,
		newFileReference,
		map[string]string{
			"processed":           "true",
			"content-disposition": fmt.Sprintf("attachment; filename=\"%s\"", input.File.Filename),
		},
	); err != nil {
		return models.Case{}, err
	}

	updatedCase, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Executor,
	) (models.Case, error) {
		if err := usecase.createCaseContributorIfNotExist(ctx, tx, input.CaseId, userId); err != nil {
			return models.Case{}, err
		}

		newCaseFileId := uuid.NewString()
		if err := usecase.repository.CreateDbCaseFile(
			ctx,
			tx,
			models.CreateDbCaseFileInput{
				Id:            newCaseFileId,
				BucketName:    usecase.gcsCaseManagerBucket,
				CaseId:        input.CaseId,
				FileName:      input.File.Filename,
				FileReference: newFileReference,
			},
		); err != nil {
			return models.Case{}, err
		}

		resourceType := models.CaseFileResourceType
		err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			CaseId:         input.CaseId,
			UserId:         userId,
			EventType:      models.CaseFileAdded,
			ResourceType:   &resourceType,
			ResourceId:     &newCaseFileId,
			AdditionalNote: &input.File.Filename,
		})
		if err != nil {
			return models.Case{}, err
		}

		return usecase.getCaseWithDetails(ctx, tx, input.CaseId)
	})
	if err != nil {
		if deleteErr := usecase.gcsRepository.DeleteFile(ctx, usecase.gcsCaseManagerBucket, newFileReference); deleteErr != nil {
			logger.WarnContext(ctx, fmt.Sprintf("failed to clean up GCS object %s after case file creation failed", newFileReference),
				"bucket", usecase.gcsCaseManagerBucket,
				"file_reference", newFileReference,
				"error", deleteErr)
			return models.Case{}, errors.Wrap(err, deleteErr.Error())
		}
		return models.Case{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsCaseFileCreated, map[string]interface{}{
		"case_id": updatedCase.Id,
	})
	return updatedCase, nil
}

func validateFileType(file *multipart.FileHeader) error {
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
	availableInboxIds, err := usecase.getAvailableInboxIds(ctx, exec)
	if err != nil {
		return "", err
	}
	if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
		return "", err
	}

	return usecase.gcsRepository.GenerateSignedUrl(ctx, usecase.gcsCaseManagerBucket, cf.FileReference)
}
