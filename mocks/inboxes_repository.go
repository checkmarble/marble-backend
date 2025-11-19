package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

type InboxRepository struct {
	mock.Mock
}

func (r *InboxRepository) ListInboxes(ctx context.Context, exec repositories.Executor,
	organizationId string, inboxIds []uuid.UUID, withCaseCount bool,
) ([]models.Inbox, error) {
	args := r.Called(ctx, exec, organizationId, inboxIds, withCaseCount)
	return args.Get(0).([]models.Inbox), args.Error(1)
}

func (r *InboxRepository) GetInboxById(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) (models.Inbox, error) {
	args := r.Called(ctx, exec, inboxId)
	return args.Get(0).(models.Inbox), args.Error(1)
}

func (r *InboxRepository) CreateInbox(ctx context.Context, exec repositories.Executor,
	input models.CreateInboxInput, newInboxId uuid.UUID,
) error {
	args := r.Called(ctx, exec, input, newInboxId)
	return args.Error(0)
}

func (r *InboxRepository) UpdateInbox(ctx context.Context, exec repositories.Executor,
	inboxId uuid.UUID, input models.UpdateInboxInput,
) error {
	args := r.Called(exec, inboxId, input)
	return args.Error(0)
}

func (r *InboxRepository) SoftDeleteInbox(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) error {
	args := r.Called(ctx, exec, inboxId)
	return args.Error(0)
}

func (r *InboxRepository) ListOrganizationCases(ctx context.Context, exec repositories.Executor,
	filters models.CaseFilters, pagination models.PaginationAndSorting,
) ([]models.Case, error) {
	args := r.Called(ctx, exec, filters, pagination)
	return args.Get(0).([]models.Case), args.Error(1)
}

func (r *InboxRepository) ListInboxUsers(ctx context.Context, exec repositories.Executor,
	filters models.InboxUserFilterInput,
) ([]models.InboxUser, error) {
	args := r.Called(ctx, exec, filters)
	return args.Get(0).([]models.InboxUser), args.Error(1)
}

func (r *InboxRepository) GetInboxUserById(ctx context.Context, exec repositories.Executor, inboxUserId uuid.UUID) (models.InboxUser, error) {
	args := r.Called(ctx, exec, inboxUserId)
	return args.Get(0).(models.InboxUser), args.Error(1)
}

func (r *InboxRepository) CreateInboxUser(ctx context.Context, exec repositories.Executor,
	input models.CreateInboxUserInput, newInboxUserId uuid.UUID,
) error {
	args := r.Called(ctx, exec, input, newInboxUserId)
	return args.Error(0)
}

func (r *InboxRepository) UpdateInboxUser(ctx context.Context, exec repositories.Executor,
	inboxUserId uuid.UUID, role *models.InboxUserRole, autoAssignable *bool,
) error {
	args := r.Called(ctx, exec, inboxUserId, role, autoAssignable)
	return args.Error(0)
}

func (r *InboxRepository) DeleteInboxUser(ctx context.Context, exec repositories.Executor, inboxUserId uuid.UUID) error {
	args := r.Called(ctx, exec, inboxUserId)
	return args.Error(0)
}
