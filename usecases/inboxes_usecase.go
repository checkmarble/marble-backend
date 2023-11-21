package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
)

type InboxRepository interface {
	GetInboxById(tx repositories.Transaction, inboxId string) (models.Inbox, error)
	ListInboxes(tx repositories.Transaction, organizationId string, inboxIds []string) ([]models.Inbox, error)
	CreateInbox(tx repositories.Transaction, createInboxAttributes models.CreateInboxInput, newInboxId string) error
	GetInboxUserById(tx repositories.Transaction, inboxUserId string) (models.InboxUser, error)
	ListInboxUsers(tx repositories.Transaction, filters models.InboxUserFilterInput) ([]models.InboxUser, error)
	CreateInboxUser(tx repositories.Transaction, createInboxUserAttributes models.CreateInboxUserInput, newInboxUserId string) error
}

type InboxUsecase struct {
	enforceSecurity security.EnforceSecurityInboxes
	// transactionFactory transaction.TransactionFactory
	repository     InboxRepository
	userRepository repositories.UserRepository
}

func (usecase *InboxUsecase) GetInboxById(ctx context.Context, inboxId string) (models.Inbox, error) {
	inbox, err := usecase.repository.GetInboxById(nil, inboxId)
	if err != nil {
		return models.Inbox{}, err
	}

	if err = usecase.enforceSecurity.ReadInbox(inbox); err != nil {
		return models.Inbox{}, err
	}

	return inbox, err
}

func (usecase *InboxUsecase) ListInboxes(ctx context.Context, organizationId string) ([]models.Inbox, error) {
	availableInboxIds, err := usecase.getAvailableInboxes(ctx)
	if err != nil {
		return []models.Inbox{}, err
	}
	if len(availableInboxIds) == 0 && availableInboxIds != nil {
		// user has access to no inboxes (as opposed to availableInboxIds==nil for org admins)
		return []models.Inbox{}, nil
	}

	inboxes, err := usecase.repository.ListInboxes(nil, organizationId, availableInboxIds)
	if err != nil {
		return []models.Inbox{}, err
	}

	for _, inbox := range inboxes {
		if err = usecase.enforceSecurity.ReadInbox(inbox); err != nil {
			return []models.Inbox{}, err
		}
	}

	return inboxes, nil
}

func (usecase *InboxUsecase) getAvailableInboxes(ctx context.Context) ([]string, error) {
	// return a slice of the inbox ids that the user has access to (can be empty)
	// or return nil if the user has access to all inboxes because he is an org admin
	availableInboxIds := make([]string, 0)

	if usecase.enforceSecurity.Credentials.Role == models.ADMIN {
		return nil, nil
	}

	userId := usecase.enforceSecurity.Credentials.ActorIdentity.UserId
	inboxUsers, err := usecase.repository.ListInboxUsers(nil, models.InboxUserFilterInput{UserId: userId})
	if err != nil {
		return []string{}, err
	}

	for _, inboxUser := range inboxUsers {
		availableInboxIds = append(availableInboxIds, inboxUser.InboxId)
	}
	return availableInboxIds, nil
}

func (usecase *InboxUsecase) CreateInbox(ctx context.Context, input models.CreateInboxInput) (models.Inbox, error) {
	if err := usecase.enforceSecurity.CreateInbox(input); err != nil {
		return models.Inbox{}, err
	}

	newInboxId := utils.NewPrimaryKey(input.OrganizationId)
	if err := usecase.repository.CreateInbox(nil, input, newInboxId); err != nil {
		return models.Inbox{}, err
	}

	inbox, err := usecase.repository.GetInboxById(nil, newInboxId)

	return inbox, err
}

func (usecase *InboxUsecase) GetInboxUserById(ctx context.Context, inboxUserId string) (models.InboxUser, error) {
	inboxUser, err := usecase.repository.GetInboxUserById(nil, inboxUserId)
	if err != nil {
		return models.InboxUser{}, err
	}

	thisUsersInboxes, err := usecase.repository.ListInboxUsers(nil, models.InboxUserFilterInput{
		UserId: usecase.enforceSecurity.Credentials.ActorIdentity.UserId,
	})
	if err != nil {
		return models.InboxUser{}, err
	}

	err = usecase.enforceSecurity.ReadInboxUser(inboxUser, thisUsersInboxes, usecase.enforceSecurity.Credentials.OrganizationId)
	if err != nil {
		return models.InboxUser{}, err
	}

	return inboxUser, nil
}

func (usecase *InboxUsecase) ListInboxUsers(ctx context.Context, inboxId string) ([]models.InboxUser, error) {
	thisUsersInboxes, err := usecase.repository.ListInboxUsers(nil, models.InboxUserFilterInput{
		UserId: usecase.enforceSecurity.Credentials.ActorIdentity.UserId,
	})
	if err != nil {
		return []models.InboxUser{}, err
	}

	inboxUsers, err := usecase.repository.ListInboxUsers(nil, models.InboxUserFilterInput{
		InboxId: inboxId,
	})
	if err != nil {
		return []models.InboxUser{}, err
	}

	for _, inboxUser := range inboxUsers {
		err = usecase.enforceSecurity.ReadInboxUser(inboxUser, thisUsersInboxes, usecase.enforceSecurity.Credentials.OrganizationId)
		if err != nil {
			return []models.InboxUser{}, err
		}
	}
	return inboxUsers, nil
}

func (usecase *InboxUsecase) CreateInboxUser(ctx context.Context, input models.CreateInboxUserInput) (models.InboxUser, error) {
	thisUsersInboxes, err := usecase.repository.ListInboxUsers(nil, models.InboxUserFilterInput{
		UserId: usecase.enforceSecurity.Credentials.ActorIdentity.UserId,
	})
	if err != nil {
		return models.InboxUser{}, err
	}

	targetUser, err := usecase.userRepository.UserByID(nil, models.UserId(input.UserId))
	if err != nil {
		return models.InboxUser{}, err
	}

	err = usecase.enforceSecurity.CreateInboxUser(input, thisUsersInboxes, usecase.enforceSecurity.Credentials.OrganizationId, targetUser)
	if err != nil {
		return models.InboxUser{}, err
	}

	newInboxUserId := utils.NewPrimaryKey(input.InboxId)
	if err := usecase.repository.CreateInboxUser(nil, input, newInboxUserId); err != nil {
		if repositories.IsUniqueViolationError(err) {
			return models.InboxUser{}, errors.Wrap(models.DuplicateValueError, "This combination of user_id and inbox_user_id already exists")
		}
		return models.InboxUser{}, err
	}

	inboxUser, err := usecase.repository.GetInboxUserById(nil, newInboxUserId)

	return inboxUser, err
}