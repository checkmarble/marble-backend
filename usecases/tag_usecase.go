package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type TagUseCaseRepository interface {
	ListOrganizationTags(ctx context.Context, tx repositories.Transaction_deprec, organizationId string, withCaseCount bool) ([]models.Tag, error)
	CreateTag(ctx context.Context, tx repositories.Transaction_deprec, attributes models.CreateTagAttributes, newTagId string) error
	UpdateTag(ctx context.Context, tx repositories.Transaction_deprec, attributes models.UpdateTagAttributes) error
	GetTagById(ctx context.Context, tx repositories.Transaction_deprec, tagId string) (models.Tag, error)
	SoftDeleteTag(ctx context.Context, tx repositories.Transaction_deprec, tagId string) error
	ListCaseTagsByTagId(ctx context.Context, tx repositories.Transaction_deprec, tagId string) ([]models.CaseTag, error)

	GetInboxById(ctx context.Context, tx repositories.Transaction_deprec, inboxId string) (models.Inbox, error)
}

type TagUseCase struct {
	enforceSecurity    security.EnforceSecurityInboxes
	transactionFactory transaction.TransactionFactory_deprec
	repository         TagUseCaseRepository
	inboxReader        inboxes.InboxReader
}

func (usecase *TagUseCase) ListAllTags(ctx context.Context, organizationId string, withCaseCount bool) ([]models.Tag, error) {
	return usecase.repository.ListOrganizationTags(ctx, nil, organizationId, withCaseCount)
}

func (usecase *TagUseCase) CreateTag(ctx context.Context, attributes models.CreateTagAttributes) (models.Tag, error) {
	if err := usecase.inboxReader.EnforceSecurity.CreateInbox(attributes.OrganizationId); err != nil {
		return models.Tag{}, err
	}

	tag, err := transaction.TransactionReturnValue_deprec(ctx,
		usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction_deprec) (models.Tag, error) {
			newTagId := uuid.NewString()
			if err := usecase.repository.CreateTag(ctx, tx, attributes, newTagId); err != nil {
				if repositories.IsUniqueViolationError(err) {
					return models.Tag{}, errors.Wrap(models.DuplicateValueError, "There is already a tag by this name")
				}
				return models.Tag{}, err
			}
			return usecase.repository.GetTagById(ctx, tx, newTagId)
		})
	if err != nil {
		return models.Tag{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsTagCreated, map[string]interface{}{"tag_id": tag.Id})

	return tag, err
}

func (usecase *TagUseCase) GetTagById(ctx context.Context, tagId string) (models.Tag, error) {
	return usecase.repository.GetTagById(ctx, nil, tagId)
}

func (usecase *TagUseCase) UpdateTag(ctx context.Context, organizationId string, attributes models.UpdateTagAttributes) (models.Tag, error) {
	if err := usecase.inboxReader.EnforceSecurity.CreateInbox(organizationId); err != nil {
		return models.Tag{}, err
	}
	tag, err := transaction.TransactionReturnValue_deprec(ctx, usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction_deprec) (models.Tag, error) {
		if err := usecase.repository.UpdateTag(ctx, tx, attributes); err != nil {
			return models.Tag{}, err
		}
		return usecase.repository.GetTagById(ctx, tx, attributes.TagId)
	})
	if err != nil {
		return models.Tag{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsTagUpdated, map[string]interface{}{"tag_id": tag.Id})

	return tag, err
}

func (usecase *TagUseCase) DeleteTag(ctx context.Context, organizationId, tagId string) error {
	if err := usecase.inboxReader.EnforceSecurity.CreateInbox(organizationId); err != nil {
		return err
	}
	err := transaction.TransactionFactory_deprec.Transaction(usecase.transactionFactory, ctx, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction_deprec) error {
		if _, err := usecase.repository.GetTagById(ctx, tx, tagId); err != nil {
			return err
		}
		caseTags, err := usecase.repository.ListCaseTagsByTagId(ctx, tx, tagId)
		if err != nil {
			return err
		}
		if len(caseTags) > 0 {
			return errors.Wrap(models.ForbiddenError, "Cannot delete tag that is still in use")
		}
		if err := usecase.repository.SoftDeleteTag(ctx, tx, tagId); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	tracking.TrackEvent(ctx, models.AnalyticsTagDeleted, map[string]interface{}{"tag_id": tagId})

	return nil
}
