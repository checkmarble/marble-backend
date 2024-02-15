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

func (r *InboxRepository) ListInboxes(ctx context.Context, exec repositories.Executor,
	organizationId string, inboxIds []string, withCaseCount bool,
) ([]models.Inbox, error) {
	args := r.Called(exec, organizationId, inboxIds)
	return args.Get(0).([]models.Inbox), args.Error(1)
}

func (r *InboxRepository) GetInboxById(ctx context.Context, exec repositories.Executor, inboxId string) (models.Inbox, error) {
	args := r.Called(exec, inboxId)
	return args.Get(0).(models.Inbox), args.Error(1)
}

func (r *InboxRepository) CreateInbox(ctx context.Context, exec repositories.Executor, input models.CreateInboxInput, newInboxId string) error {
	args := r.Called(exec, input, newInboxId)
	return args.Error(0)
}

func (r *InboxRepository) UpdateInbox(ctx context.Context, exec repositories.Executor, inboxId, name string) error {
	args := r.Called(exec, inboxId, name)
	return args.Error(0)
}

func (r *InboxRepository) SoftDeleteInbox(ctx context.Context, exec repositories.Executor, inboxId string) error {
	args := r.Called(exec, inboxId)
	return args.Error(0)
}

func (r *InboxRepository) ListOrganizationCases(ctx context.Context, exec repositories.Executor,
	filters models.CaseFilters, pagination models.PaginationAndSorting,
) ([]models.CaseWithRank, error) {
	args := r.Called(exec, filters, pagination)
	return args.Get(0).([]models.CaseWithRank), args.Error(1)
}

func (r *InboxRepository) ListInboxUsers(ctx context.Context, exec repositories.Executor,
	filters models.InboxUserFilterInput,
) ([]models.InboxUser, error) {
	args := r.Called(exec, filters)
	return args.Get(0).([]models.InboxUser), args.Error(1)
}

func (repo *InboxRepository) GetInboxUserById(ctx context.Context, exec repositories.Executor, inboxUserId string) (models.InboxUser, error) {
	args := repo.Called(exec, inboxUserId)
	return args.Get(0).(models.InboxUser), args.Error(1)
}

func (repo *InboxRepository) CreateInboxUser(ctx context.Context, exec repositories.Executor,
	input models.CreateInboxUserInput, newInboxUserId string,
) error {
	args := repo.Called(exec, input, newInboxUserId)
	return args.Error(0)
}

func (repo *InboxRepository) UpdateInboxUser(ctx context.Context, exec repositories.Executor, inboxUserId string, role models.InboxUserRole) error {
	args := repo.Called(exec, inboxUserId, role)
	return args.Error(0)
}

func (repo *InboxRepository) DeleteInboxUser(ctx context.Context, exec repositories.Executor, inboxUserId string) error {
	args := repo.Called(exec, inboxUserId)
	return args.Error(0)
}
