package organization

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateOrganizationMapsUniqueViolationToConflict(t *testing.T) {
	ctx := context.Background()
	orgRepository := &mocks.OrganizationRepository{}
	orgRepository.On("CreateOrganization", ctx, mock.Anything, mock.Anything, mock.Anything).
		Return(&pgconn.PgError{Code: pgerrcode.UniqueViolation})

	txFactory := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
	txFactory.On("Transaction", ctx, mock.Anything).Return(nil)

	creator := OrganizationCreator{
		OrganizationRepository: orgRepository,
		TransactionFactory:     txFactory,
	}
	_, err := creator.CreateOrganization(ctx, models.CreateOrganizationInput{Name: "Acme"})

	require.ErrorIs(t, err, models.ConflictError)
	require.ErrorContains(t, err, "organization or tenant with the same name")
	orgRepository.AssertExpectations(t)
	txFactory.AssertExpectations(t)
}
