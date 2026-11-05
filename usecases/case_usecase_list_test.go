package usecases

import (
	"context"
	"slices"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestListCases_ForwardsTypeFilterToRepository(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.New()
	inboxId := uuid.New()

	tx := new(mocks.Transaction)
	transactionFactory := &mocks.TransactionFactory{TxMock: tx}
	transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)

	inbox := models.Inbox{Id: inboxId, OrganizationId: orgId}
	security := new(mocks.EnforceSecurity)
	security.On("ReadInbox", inbox).Return(nil)
	inboxRepository := new(mocks.InboxRepository)
	inboxRepository.On("ListInboxes", ctx, tx, orgId, []uuid.UUID(nil), false).Return([]models.Inbox{inbox}, nil)

	repository := new(mocks.CaseRepository)
	repository.On("ListOrganizationCases", ctx, tx,
		mock.MatchedBy(func(filters models.CaseFilters) bool {
			return slices.Equal(filters.Types, []models.CaseType{models.CaseTypeContinuousScreening})
		}),
		mock.Anything,
	).Return([]models.Case{}, nil)

	usecase := CaseUseCase{
		repository: repository,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity: security,
			InboxRepository: inboxRepository,
			Credentials:     models.Credentials{Roles: []models.Role{models.ADMIN}},
		},
		transactionFactory: transactionFactory,
	}

	_, err := usecase.ListCases(ctx, orgId,
		models.PaginationAndSorting{Limit: 25, Order: models.SortingOrderDesc, Sorting: models.CasesSortingCreatedAt},
		models.CaseFilters{Types: []models.CaseType{models.CaseTypeContinuousScreening}},
	)

	require.NoError(t, err)
	repository.AssertExpectations(t)
}
