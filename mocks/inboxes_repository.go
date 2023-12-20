package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type InboxRepository struct {
	mock.Mock
}

func (r *InboxRepository) ListInboxes(tx repositories.Transaction, organizationId string, inboxIds []string, withCaseCount bool) ([]models.Inbox, error) {
	args := r.Called(tx, organizationId, inboxIds)
	return args.Get(0).([]models.Inbox), args.Error(1)
}

func (r *InboxRepository) GetInboxById(tx repositories.Transaction, inboxId string) (models.Inbox, error) {
	args := r.Called(tx, inboxId)
	return args.Get(0).(models.Inbox), args.Error(1)
}

func (r *InboxRepository) CreateInbox(tx repositories.Transaction, input models.CreateInboxInput, newInboxId string) error {
	args := r.Called(tx, input, newInboxId)
	return args.Error(0)
}

func (r *InboxRepository) UpdateInbox(tx repositories.Transaction, inboxId, name string) error {
	args := r.Called(tx, inboxId, name)
	return args.Error(0)
}

func (r *InboxRepository) SoftDeleteInbox(tx repositories.Transaction, inboxId string) error {
	args := r.Called(tx, inboxId)
	return args.Error(0)
}

func (r *InboxRepository) ListOrganizationCases(tx repositories.Transaction, filters models.CaseFilters, pagination models.PaginationAndSorting) ([]models.CaseWithRank, error) {
	args := r.Called(tx, filters, pagination)
	return args.Get(0).([]models.CaseWithRank), args.Error(1)
}

func (r *InboxRepository) ListInboxUsers(tx repositories.Transaction, filters models.InboxUserFilterInput) ([]models.InboxUser, error) {
	args := r.Called(tx, filters)
	return args.Get(0).([]models.InboxUser), args.Error(1)
}

func (repo *InboxRepository) GetInboxUserById(tx repositories.Transaction, inboxUserId string) (models.InboxUser, error) {
	args := repo.Called(tx, inboxUserId)
	return args.Get(0).(models.InboxUser), args.Error(1)
}

func (repo *InboxRepository) CreateInboxUser(tx repositories.Transaction, input models.CreateInboxUserInput, newInboxUserId string) error {
	args := repo.Called(tx, input, newInboxUserId)
	return args.Error(0)
}

func (repo *InboxRepository) UpdateInboxUser(tx repositories.Transaction, inboxUserId string, role models.InboxUserRole) error {
	args := repo.Called(tx, inboxUserId, role)
	return args.Error(0)
}

func (repo *InboxRepository) DeleteInboxUser(tx repositories.Transaction, inboxUserId string) error {
	args := repo.Called(tx, inboxUserId)
	return args.Error(0)
}
