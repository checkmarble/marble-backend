package usecases

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/auth"
)

type onboardingTestDeps struct {
	exec               *mocks.Executor
	transaction        *mocks.Transaction
	transactionFactory *mocks.TransactionFactory
	executorFactory    *mocks.ExecutorFactory
	orgRepository      *mocks.OrganizationRepository
	userRepository     *mocks.UserRepository
	grantRepository    *mocks.GrantRepository
	firebase           *mocks.FirebaseAdminClient
}

func onboardingUsecaseForTest(tokenProvider auth.TokenProvider) (OnboardingUsecase, onboardingTestDeps) {
	deps := onboardingTestDeps{
		exec:            new(mocks.Executor),
		transaction:     new(mocks.Transaction),
		executorFactory: new(mocks.ExecutorFactory),
		orgRepository:   new(mocks.OrganizationRepository),
		userRepository:  new(mocks.UserRepository),
		grantRepository: new(mocks.GrantRepository),
		firebase:        new(mocks.FirebaseAdminClient),
	}
	deps.transactionFactory = &mocks.TransactionFactory{TxMock: deps.transaction}

	uc := NewOnboardingUsecase(
		deps.executorFactory,
		deps.transactionFactory,
		deps.orgRepository,
		deps.userRepository,
		deps.grantRepository,
		tokenProvider,
		deps.firebase,
	)

	return uc, deps
}

// expectAdvisoryLock stubs the pg_advisory_xact_lock call issued by GetAdvisoryLockTx.
func (deps onboardingTestDeps) expectAdvisoryLock() {
	deps.transaction.On("Exec", mock.Anything, "SELECT pg_advisory_xact_lock($1)", mock.Anything).
		Return(pgconn.NewCommandTag("SELECT 1"), nil)
}

func validOnboardingPayload() dto.CreateInitialOrg {
	return dto.CreateInitialOrg{
		Organization: "Acme",
		Email:        "admin@acme.com",
		Firstname:    "Ada",
		Lastname:     "Lovelace",
	}
}

func TestCreateInitialOrganization(t *testing.T) {
	ctx := context.Background()

	t.Run("creates the organization and its admin user when the instance is empty", func(t *testing.T) {
		uc, deps := onboardingUsecaseForTest(auth.TokenProviderOidc)
		deps.expectAdvisoryLock()
		deps.executorFactory.On("NewExecutor").Return(deps.exec)
		deps.orgRepository.On("HasOrganizations", ctx, deps.exec).Return(false, nil)
		deps.orgRepository.On("HasOrganizations", ctx, deps.transaction).Return(false, nil)
		deps.orgRepository.On("CreateOrganization", ctx, deps.transaction, mock.Anything,
			models.CreateOrganizationInput{Name: "Acme"}).Return(nil)
		deps.userRepository.On("CreateUser", ctx, deps.transaction, mock.MatchedBy(
			func(u models.CreateUser) bool {
				return u.Email == "admin@acme.com" && u.Role == models.ADMIN
			},
		)).Return("some-user-id", nil)
		deps.grantRepository.On("EnsureTenantAdminForOrganization", ctx, deps.transaction, "some-user-id", mock.Anything).Return(nil)
		deps.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)

		require.NoError(t, uc.CreateInitialOrganization(ctx, validOnboardingPayload()))

		deps.orgRepository.AssertExpectations(t)
		deps.userRepository.AssertExpectations(t)
		deps.grantRepository.AssertExpectations(t)
	})

	t.Run("normalizes the email before persisting it", func(t *testing.T) {
		uc, deps := onboardingUsecaseForTest(auth.TokenProviderOidc)
		deps.expectAdvisoryLock()
		deps.executorFactory.On("NewExecutor").Return(deps.exec)
		deps.orgRepository.On("HasOrganizations", mock.Anything, mock.Anything).Return(false, nil)
		deps.orgRepository.On("CreateOrganization", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything).Return(nil)
		deps.userRepository.On("CreateUser", ctx, deps.transaction, mock.MatchedBy(
			func(u models.CreateUser) bool { return u.Email == "admin@acme.com" },
		)).
			Return("some-user-id", nil)
		deps.grantRepository.On("EnsureTenantAdminForOrganization", mock.Anything, mock.Anything, "some-user-id", mock.Anything).Return(nil)
		deps.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)

		payload := validOnboardingPayload()
		payload.Email = "  Admin@Acme.COM "

		require.NoError(t, uc.CreateInitialOrganization(ctx, payload))

		deps.userRepository.AssertExpectations(t)
	})

	t.Run("rejects onboarding when an organization already exists", func(t *testing.T) {
		uc, deps := onboardingUsecaseForTest(auth.TokenProviderOidc)
		deps.executorFactory.On("NewExecutor").Return(deps.exec)
		deps.orgRepository.On("HasOrganizations", ctx, deps.exec).Return(true, nil)

		err := uc.CreateInitialOrganization(ctx, validOnboardingPayload())

		assert.ErrorIs(t, err, models.ConflictError)
		deps.orgRepository.AssertNotCalled(t, "CreateOrganization")
		deps.userRepository.AssertNotCalled(t, "CreateUser")
	})

	// The authoritative check: the pre-check saw an empty instance, but a concurrent request won
	// the race and committed before this one acquired the advisory lock.
	t.Run("rejects onboarding when a concurrent request created the organization", func(t *testing.T) {
		uc, deps := onboardingUsecaseForTest(auth.TokenProviderOidc)
		deps.expectAdvisoryLock()
		deps.executorFactory.On("NewExecutor").Return(deps.exec)
		deps.orgRepository.On("HasOrganizations", ctx, deps.exec).Return(false, nil)
		deps.orgRepository.On("HasOrganizations", ctx, deps.transaction).Return(true, nil)
		deps.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)

		err := uc.CreateInitialOrganization(ctx, validOnboardingPayload())

		assert.ErrorIs(t, err, models.ConflictError)
		deps.orgRepository.AssertNotCalled(t, "CreateOrganization")
		deps.userRepository.AssertNotCalled(t, "CreateUser")
	})

	// Regression test: the instance used to be considered empty when the tables did not exist.
	t.Run("maps a duplicate organization name to a conflict", func(t *testing.T) {
		uc, deps := onboardingUsecaseForTest(auth.TokenProviderOidc)
		deps.expectAdvisoryLock()
		deps.executorFactory.On("NewExecutor").Return(deps.exec)
		deps.orgRepository.On("HasOrganizations", mock.Anything, mock.Anything).Return(false, nil)
		deps.orgRepository.On("CreateOrganization", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything).Return(&pgconn.PgError{Code: pgerrcode.UniqueViolation})
		deps.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)

		err := uc.CreateInitialOrganization(ctx, validOnboardingPayload())

		assert.ErrorIs(t, err, models.ConflictError)
		deps.userRepository.AssertNotCalled(t, "CreateUser")
	})

	t.Run("maps a duplicate user email to a conflict", func(t *testing.T) {
		uc, deps := onboardingUsecaseForTest(auth.TokenProviderOidc)
		deps.expectAdvisoryLock()
		deps.executorFactory.On("NewExecutor").Return(deps.exec)
		deps.orgRepository.On("HasOrganizations", mock.Anything, mock.Anything).Return(false, nil)
		deps.orgRepository.On("CreateOrganization", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything).Return(nil)
		deps.userRepository.On("CreateUser", mock.Anything, mock.Anything, mock.Anything).
			Return("", &pgconn.PgError{Code: pgerrcode.UniqueViolation})
		deps.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)

		err := uc.CreateInitialOrganization(ctx, validOnboardingPayload())

		assert.ErrorIs(t, err, models.ConflictError)
	})

	t.Run("requires a password when configured to use Firebase", func(t *testing.T) {
		uc, deps := onboardingUsecaseForTest(auth.TokenProviderFirebase)

		err := uc.CreateInitialOrganization(ctx, validOnboardingPayload())

		assert.ErrorIs(t, err, models.BadParameterError)
		deps.executorFactory.AssertNotCalled(t, "NewExecutor")
		deps.orgRepository.AssertNotCalled(t, "CreateOrganization")
		deps.userRepository.AssertNotCalled(t, "CreateUser")
	})

	// The identity provider account is created before the transaction opens, so that no network
	// call is made while holding a pooled connection and the advisory lock.
	t.Run("creates the Firebase user before opening the transaction", func(t *testing.T) {
		uc, deps := onboardingUsecaseForTest(auth.TokenProviderFirebase)
		firebaseCalled := false
		deps.expectAdvisoryLock()
		deps.executorFactory.On("NewExecutor").Return(deps.exec)
		deps.orgRepository.On("HasOrganizations", mock.Anything, mock.Anything).Return(false, nil)
		deps.firebase.On("CreateFirstUser", ctx, "admin@acme.com", "hunter2hunter2", "Ada Lovelace").
			Return(nil).Run(func(mock.Arguments) { firebaseCalled = true })
		deps.transactionFactory.On("Transaction", ctx, mock.Anything).
			Return(nil).
			Run(func(mock.Arguments) {
				assert.True(t, firebaseCalled, "Firebase user must be created before the transaction opens")
			})
		deps.orgRepository.On("CreateOrganization", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything).Return(nil)
		deps.userRepository.On("CreateUser", mock.Anything, mock.Anything, mock.Anything).
			Return("some-user-id", nil)
		deps.grantRepository.On("EnsureTenantAdminForOrganization", mock.Anything, mock.Anything, "some-user-id", mock.Anything).Return(nil)

		payload := validOnboardingPayload()
		payload.Password = "hunter2hunter2"

		require.NoError(t, uc.CreateInitialOrganization(ctx, payload))

		deps.firebase.AssertExpectations(t)
	})

	t.Run("does not create the organization when the Firebase user cannot be created", func(t *testing.T) {
		uc, deps := onboardingUsecaseForTest(auth.TokenProviderFirebase)
		deps.executorFactory.On("NewExecutor").Return(deps.exec)
		deps.orgRepository.On("HasOrganizations", ctx, deps.exec).Return(false, nil)
		deps.firebase.On("CreateFirstUser", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything).Return(errors.New("firebase is down"))

		payload := validOnboardingPayload()
		payload.Password = "hunter2hunter2"

		err := uc.CreateInitialOrganization(ctx, payload)

		require.Error(t, err)
		deps.transactionFactory.AssertNotCalled(t, "Transaction")
		deps.orgRepository.AssertNotCalled(t, "CreateOrganization")
	})

	t.Run("does not touch the identity provider on an already onboarded instance", func(t *testing.T) {
		uc, deps := onboardingUsecaseForTest(auth.TokenProviderFirebase)
		deps.executorFactory.On("NewExecutor").Return(deps.exec)
		deps.orgRepository.On("HasOrganizations", ctx, deps.exec).Return(true, nil)

		payload := validOnboardingPayload()
		payload.Password = "hunter2hunter2"

		err := uc.CreateInitialOrganization(ctx, payload)

		assert.ErrorIs(t, err, models.ConflictError)
		deps.firebase.AssertNotCalled(t, "CreateFirstUser")
		deps.transactionFactory.AssertNotCalled(t, "Transaction")
	})

	t.Run("propagates repository errors that are not conflicts", func(t *testing.T) {
		uc, deps := onboardingUsecaseForTest(auth.TokenProviderOidc)
		expected := errors.New("boom")
		deps.expectAdvisoryLock()
		deps.executorFactory.On("NewExecutor").Return(deps.exec)
		deps.orgRepository.On("HasOrganizations", mock.Anything, mock.Anything).Return(false, nil)
		deps.orgRepository.On("CreateOrganization", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything).Return(expected)
		deps.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)

		err := uc.CreateInitialOrganization(ctx, validOnboardingPayload())

		assert.ErrorIs(t, err, expected)
		assert.NotErrorIs(t, err, models.ConflictError)
	})
}
