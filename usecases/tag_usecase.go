package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type TagUseCaseRepository interface {
	ListOrganizationTags(ctx context.Context, exec repositories.Executor, organizationId string,
		withCaseCount bool) ([]models.Tag, error)
	CreateTag(ctx context.Context, exec repositories.Executor, attributes models.CreateTagAttributes, newTagId string) error
	UpdateTag(ctx context.Context, exec repositories.Executor, attributes models.UpdateTagAttributes) error
	GetTagById(ctx context.Context, exec repositories.Executor, tagId string) (models.Tag, error)
	SoftDeleteTag(ctx context.Context, exec repositories.Executor, tagId string) error
	ListCaseTagsByTagId(ctx context.Context, exec repositories.Executor, tagId string) ([]models.CaseTag, error)
}

type TagUseCase struct {
	enforceSecurity    security.EnforceSecurityTags
	transactionFactory executor_factory.TransactionFactory
	executorFactory    executor_factory.ExecutorFactory
	repository         TagUseCaseRepository
}

func (usecase *TagUseCase) ListAllTags(ctx context.Context, organizationId string, withCaseCount bool) ([]models.Tag, error) {
	tags, err := usecase.repository.ListOrganizationTags(
		ctx,
		usecase.executorFactory.NewExecutor(),
		organizationId,
		withCaseCount)
	if err != nil {
		return nil, err
	}

	for _, t := range tags {
		if err := usecase.enforceSecurity.ReadTag(t); err != nil {
			return nil, err
		}
	}
	return tags, err
}

func (usecase *TagUseCase) CreateTag(ctx context.Context, attributes models.CreateTagAttributes) (models.Tag, error) {
	if err := usecase.enforceSecurity.CreateTag(attributes.OrganizationId); err != nil {
		return models.Tag{}, err
	}

	tag, err := executor_factory.TransactionReturnValue(ctx,
		usecase.transactionFactory, func(tx repositories.Transaction) (models.Tag, error) {
			newTagId := uuid.NewString()
			if err := usecase.repository.CreateTag(ctx, tx, attributes, newTagId); err != nil {
				if repositories.IsUniqueViolationError(err) {
					return models.Tag{}, errors.Wrap(models.ConflictError, "There is already a tag by this name")
				}
				return models.Tag{}, err
			}
			return usecase.repository.GetTagById(ctx, tx, newTagId)
		})
	if err != nil {
		return models.Tag{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsTagCreated, map[string]interface{}{
		"tag_id": tag.Id,
	})

	return tag, err
}

func (usecase *TagUseCase) GetTagById(ctx context.Context, tagId string) (models.Tag, error) {
	t, err := usecase.repository.GetTagById(ctx, usecase.executorFactory.NewExecutor(), tagId)
	if err != nil {
		return models.Tag{}, err
	}
	if err := usecase.enforceSecurity.ReadTag(t); err != nil {
		return models.Tag{}, err
	}
	return t, nil
}

func (usecase *TagUseCase) UpdateTag(ctx context.Context, attributes models.UpdateTagAttributes) (models.Tag, error) {
	tag, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.Tag, error) {
		tag, err := usecase.repository.GetTagById(ctx, tx, attributes.TagId)
		if err != nil {
			return models.Tag{}, err
		}
		if err := usecase.enforceSecurity.UpdateTag(tag); err != nil {
			return models.Tag{}, err
		}

		if err := usecase.repository.UpdateTag(ctx, tx, attributes); err != nil {
			return models.Tag{}, err
		}
		return usecase.repository.GetTagById(ctx, tx, attributes.TagId)
	})
	if err != nil {
		return models.Tag{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsTagUpdated, map[string]interface{}{
		"tag_id": tag.Id,
	})

	return tag, err
}

func (usecase *TagUseCase) DeleteTag(ctx context.Context, organizationId, tagId string) error {
	err := executor_factory.TransactionFactory.Transaction(usecase.transactionFactory, ctx, func(tx repositories.Transaction) error {
		t, err := usecase.repository.GetTagById(ctx, tx, tagId)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.DeleteTag(t); err != nil {
			return err
		}

		caseTags, err := usecase.repository.ListCaseTagsByTagId(ctx, tx, tagId)
		if err != nil {
			return err
		}
		if len(caseTags) > 0 {
			return errors.Wrap(models.ForbiddenError,
				"Cannot delete tag that is still in use")
		}
		if err := usecase.repository.SoftDeleteTag(ctx, tx, tagId); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	tracking.TrackEvent(ctx, models.AnalyticsTagDeleted, map[string]interface{}{
		"tag_id": tagId,
	})

	return nil
}
