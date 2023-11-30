package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/analytics"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type TagUseCaseRepository interface {
	ListOrganizationTags(tx repositories.Transaction, organizationId string) ([]models.Tag, error)
	CreateTag(tx repositories.Transaction, attributes models.CreateTagAttributes, newTagId string) error
	UpdateTag(tx repositories.Transaction, attributes models.UpdateTagAttributes) error
	GetTagById(tx repositories.Transaction, tagId string) (models.Tag, error)
	SoftDeleteTag(tx repositories.Transaction, tagId string) error

	GetInboxById(tx repositories.Transaction, inboxId string) (models.Inbox, error)
}

type TagUseCase struct {
	enforceSecurity    security.EnforceSecurityInboxes
	transactionFactory transaction.TransactionFactory
	repository         TagUseCaseRepository
	inboxReader        inboxes.InboxReader
}

func (usecase *TagUseCase) ListAllTags(ctx context.Context, organizationId string) ([]models.Tag, error) {
	return usecase.repository.ListOrganizationTags(nil, organizationId)
}

func (usecase *TagUseCase) CreateTag(ctx context.Context, attributes models.CreateTagAttributes) (models.Tag, error) {
	if err := usecase.inboxReader.EnforceSecurity.CreateInbox(attributes.OrganizationId); err != nil {
		return models.Tag{}, err
	}

	tag, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Tag, error) {
		newTagId := uuid.NewString()
		if err := usecase.repository.CreateTag(tx, attributes, newTagId); err != nil {
			if repositories.IsUniqueViolationError(err) {
				return models.Tag{}, errors.Wrap(models.DuplicateValueError, "There is already a tag by this name")
			}
			return models.Tag{}, err
		}
		return usecase.repository.GetTagById(tx, newTagId)
	})

	analytics.TrackEvent(ctx, models.AnalyticsTagCreated, map[string]interface{}{"tag_id": tag.Id})

	return tag, err
}

func (usecase *TagUseCase) GetTagById(tagId string) (models.Tag, error) {
	return usecase.repository.GetTagById(nil, tagId)
}

func (usecase *TagUseCase) UpdateTag(ctx context.Context, organizationId string, attributes models.UpdateTagAttributes) (models.Tag, error) {
	if err := usecase.inboxReader.EnforceSecurity.CreateInbox(organizationId); err != nil {
		return models.Tag{}, err
	}
	tag, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Tag, error) {
		if err := usecase.repository.UpdateTag(tx, attributes); err != nil {
			return models.Tag{}, err
		}
		return usecase.repository.GetTagById(tx, attributes.TagId)
	})

	analytics.TrackEvent(ctx, models.AnalyticsTagUpdated, map[string]interface{}{"tag_id": tag.Id})

	return tag, err
}

func (usecase *TagUseCase) DeleteTag(ctx context.Context, organizationId, tagId string) error {
	if err := usecase.inboxReader.EnforceSecurity.CreateInbox(organizationId); err != nil {
		return err
	}
	err := transaction.TransactionFactory.Transaction(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		if _, err := usecase.repository.GetTagById(tx, tagId); err != nil {
			return err
		}
		if err := usecase.repository.SoftDeleteTag(tx, tagId); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	analytics.TrackEvent(ctx, models.AnalyticsTagDeleted, map[string]interface{}{"tag_id": tagId})

	return nil
}
