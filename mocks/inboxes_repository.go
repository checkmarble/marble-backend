package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type InboxRepository struct {
	mock.Mock
}

func (r *InboxRepository) ListInboxes(ctx context.Context, tx repositories.Transaction_deprec, organizationId string, inboxIds []string, withCaseCount bool) ([]models.Inbox, error) {
	args := r.Called(tx, organizationId, inboxIds)
	return args.Get(0).([]models.Inbox), args.Error(1)
}

func (r *InboxRepository) GetInboxById(ctx context.Context, tx repositories.Transaction_deprec, inboxId string) (models.Inbox, error) {
	args := r.Called(tx, inboxId)
	return args.Get(0).(models.Inbox), args.Error(1)
}

func (r *InboxRepository) CreateInbox(ctx context.Context, tx repositories.Transaction_deprec, input models.CreateInboxInput, newInboxId string) error {
	args := r.Called(tx, input, newInboxId)
	return args.Error(0)
}

func (r *InboxRepository) UpdateInbox(ctx context.Context, tx repositories.Transaction_deprec, inboxId, name string) error {
	args := r.Called(tx, inboxId, name)
	return args.Error(0)
}

func (r *InboxRepository) SoftDeleteInbox(ctx context.Context, tx repositories.Transaction_deprec, inboxId string) error {
	args := r.Called(tx, inboxId)
	return args.Error(0)
}

func (r *InboxRepository) ListOrganizationCases(ctx context.Context, tx repositories.Transaction_deprec, filters models.CaseFilters, pagination models.PaginationAndSorting) ([]models.CaseWithRank, error) {
	args := r.Called(tx, filters, pagination)
	return args.Get(0).([]models.CaseWithRank), args.Error(1)
}

func (r *InboxRepository) ListInboxUsers(ctx context.Context, tx repositories.Transaction_deprec, filters models.InboxUserFilterInput) ([]models.InboxUser, error) {
	args := r.Called(tx, filters)
	return args.Get(0).([]models.InboxUser), args.Error(1)
}

func (repo *InboxRepository) GetInboxUserById(ctx context.Context, tx repositories.Transaction_deprec, inboxUserId string) (models.InboxUser, error) {
	args := repo.Called(tx, inboxUserId)
	return args.Get(0).(models.InboxUser), args.Error(1)
}

func (repo *InboxRepository) CreateInboxUser(ctx context.Context, tx repositories.Transaction_deprec, input models.CreateInboxUserInput, newInboxUserId string) error {
	args := repo.Called(tx, input, newInboxUserId)
	return args.Error(0)
}

func (repo *InboxRepository) UpdateInboxUser(ctx context.Context, tx repositories.Transaction_deprec, inboxUserId string, role models.InboxUserRole) error {
	args := repo.Called(tx, inboxUserId, role)
	return args.Error(0)
}

func (repo *InboxRepository) DeleteInboxUser(ctx context.Context, tx repositories.Transaction_deprec, inboxUserId string) error {
	args := repo.Called(tx, inboxUserId)
	return args.Error(0)
}
