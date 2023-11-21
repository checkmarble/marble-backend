package repositories

import "github.com/checkmarble/marble-backend/models"

func (repo *MarbleDbRepository) GetInboxById(tx Transaction, inboxId string) (models.Inbox, error) {
	return models.Inbox{}, nil
}

func (repo *MarbleDbRepository) ListInboxes(tx Transaction, organizationId string, inboxIds []string) ([]models.Inbox, error) {
	return []models.Inbox{}, nil
}

func (repo *MarbleDbRepository) CreateInbox(tx Transaction, createInboxAttributes models.CreateInboxInput, newInboxId string) error {
	return nil
}

func (repo *MarbleDbRepository) GetInboxUserById(tx Transaction, inboxUserId string) (models.InboxUser, error) {
	return models.InboxUser{}, nil
}

func (repo *MarbleDbRepository) ListInboxUsers(tx Transaction, filters models.InboxUserFilterInput) ([]models.InboxUser, error) {
	return []models.InboxUser{}, nil
}

func (repo *MarbleDbRepository) CreateInboxUser(tx Transaction, createInboxUserAttributes models.CreateInboxUserInput, newInboxUserId string) error {
	return nil
}
