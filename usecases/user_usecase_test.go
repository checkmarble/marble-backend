package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type allowOrganizationGrantSecurity struct{}

func (allowOrganizationGrantSecurity) Permission(models.Permission) error              { return nil }
func (allowOrganizationGrantSecurity) ReadOrganization(uuid.UUID) error                { return nil }
func (allowOrganizationGrantSecurity) Permissions([]models.Permission) error           { return nil }
func (allowOrganizationGrantSecurity) OrgId() uuid.UUID                                { return uuid.Nil }
func (allowOrganizationGrantSecurity) UserId() *string                                 { return nil }
func (allowOrganizationGrantSecurity) ApiKeyId() *string                               { return nil }
func (allowOrganizationGrantSecurity) ReadUser(models.User) error                      { return nil }
func (allowOrganizationGrantSecurity) CreateUser(models.CreateUser) error              { return nil }
func (allowOrganizationGrantSecurity) UpdateUser(models.User, models.UpdateUser) error { return nil }
func (allowOrganizationGrantSecurity) DeleteUser(models.User) error                    { return nil }
func (allowOrganizationGrantSecurity) ListUsers(*uuid.UUID) error                      { return nil }
func (allowOrganizationGrantSecurity) ListTenantUsers(uuid.UUID) error                 { return nil }
func (allowOrganizationGrantSecurity) ManageOrganizationGrant(uuid.UUID, models.User) error {
	return nil
}
func (allowOrganizationGrantSecurity) ListUserGrants() error { return nil }

type denyListUserGrantsSecurity struct{ allowOrganizationGrantSecurity }

func (denyListUserGrantsSecurity) ListUserGrants() error { return models.ForbiddenError }

var _ security.EnforceSecurityUser = allowOrganizationGrantSecurity{}

func TestUserUseCaseReplaceOrganizationGrant(t *testing.T) {
	userID := uuid.NewString()
	homeOrganizationID := uuid.New()
	targetOrganizationID := uuid.New()
	tenantID := uuid.New()

	tests := []struct {
		name       string
		tenant     uuid.UUID
		homeTenant uuid.UUID
		userRole   models.Role
		homeOrg    uuid.UUID
		wantError  error
	}{
		{name: "same tenant", tenant: tenantID, homeTenant: tenantID, homeOrg: homeOrganizationID},
		{name: "route tenant differs from organization", tenant: uuid.New(), homeTenant: tenantID, homeOrg: homeOrganizationID, wantError: models.NotFoundError},
		{name: "different tenant", tenant: tenantID, homeTenant: uuid.New(), homeOrg: homeOrganizationID, wantError: models.NotFoundError},
		{name: "platform user", tenant: tenantID, userRole: models.MARBLE_ADMIN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transactionFactory := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
			userRepository := &mocks.UserRepository{}
			grantRepository := &mocks.GrantRepository{}
			organizationRepository := &mocks.OrganizationRepository{}

			userRepository.On("UserById", mock.Anything, mock.Anything, userID).Return(models.User{OrganizationId: tt.homeOrg, Role: tt.userRole}, nil)
			organizationRepository.On("GetOrganizationById", mock.Anything, mock.Anything, targetOrganizationID).Return(models.Organization{Id: targetOrganizationID, TenantId: tenantID}, nil)
			if tt.homeOrg != uuid.Nil {
				organizationRepository.On("GetOrganizationById", mock.Anything, mock.Anything, tt.homeOrg).Return(models.Organization{Id: tt.homeOrg, TenantId: tt.homeTenant}, nil)
			}
			transactionFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
			if tt.wantError == nil {
				grantRepository.On("ReplaceOrganizationGrant", mock.Anything, mock.Anything, userID, targetOrganizationID, models.VIEWER).Return(nil)
			}

			usecase := UserUseCase{
				enforceUserSecurity:    allowOrganizationGrantSecurity{},
				transactionFactory:     transactionFactory,
				userRepository:         userRepository,
				grantRepository:        grantRepository,
				organizationRepository: organizationRepository,
			}
			err := usecase.ReplaceOrganizationGrant(context.Background(), userID, tt.tenant, targetOrganizationID, models.VIEWER)
			if tt.wantError != nil {
				require.ErrorIs(t, err, tt.wantError)
				grantRepository.AssertNotCalled(t, "ReplaceOrganizationGrant", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				return
			}
			require.NoError(t, err)
			grantRepository.AssertExpectations(t)
			transactionFactory.AssertExpectations(t)
		})
	}
}

func TestUserUseCaseRevokePlatformUserGrant(t *testing.T) {
	userID := uuid.NewString()
	organizationID := uuid.New()
	tenantID := uuid.New()
	transactionFactory := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
	userRepository := &mocks.UserRepository{}
	grantRepository := &mocks.GrantRepository{}
	organizationRepository := &mocks.OrganizationRepository{}

	userRepository.On("UserById", mock.Anything, mock.Anything, userID).
		Return(models.User{Role: models.MARBLE_ADMIN}, nil)
	organizationRepository.On("GetOrganizationById", mock.Anything, mock.Anything, organizationID).
		Return(models.Organization{Id: organizationID, TenantId: tenantID}, nil)
	transactionFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
	grantRepository.On("RevokeOrganizationGrant", mock.Anything, mock.Anything, userID, organizationID).
		Return(nil)

	usecase := UserUseCase{
		enforceUserSecurity:    allowOrganizationGrantSecurity{},
		transactionFactory:     transactionFactory,
		userRepository:         userRepository,
		grantRepository:        grantRepository,
		organizationRepository: organizationRepository,
	}

	require.NoError(t, usecase.RevokeOrganizationGrant(context.Background(), userID, tenantID, organizationID))
	grantRepository.AssertExpectations(t)
}

func TestUserUseCaseRevokeOrganizationGrantRejectsOrganizationOutsideRouteTenant(t *testing.T) {
	userID := uuid.NewString()
	organizationID := uuid.New()
	organizationTenantID := uuid.New()

	transactionFactory := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
	userRepository := &mocks.UserRepository{}
	grantRepository := &mocks.GrantRepository{}
	organizationRepository := &mocks.OrganizationRepository{}

	organizationRepository.On("GetOrganizationById", mock.Anything, mock.Anything, organizationID).
		Return(models.Organization{Id: organizationID, TenantId: organizationTenantID}, nil)
	transactionFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)

	usecase := UserUseCase{
		enforceUserSecurity:    allowOrganizationGrantSecurity{},
		transactionFactory:     transactionFactory,
		userRepository:         userRepository,
		grantRepository:        grantRepository,
		organizationRepository: organizationRepository,
	}

	err := usecase.RevokeOrganizationGrant(context.Background(), userID, uuid.New(), organizationID)
	require.ErrorIs(t, err, models.NotFoundError)
	userRepository.AssertNotCalled(t, "UserById", mock.Anything, mock.Anything, mock.Anything)
	grantRepository.AssertNotCalled(t, "RevokeOrganizationGrant", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestUserUseCaseListUserGrants(t *testing.T) {
	userID := uuid.NewString()
	grants := []models.Grant{
		{Role: models.VIEWER, TenantId: uuid.New(), OrganizationId: uuid.New()},
		{Role: models.ADMIN, TenantId: uuid.New(), OrganizationId: uuid.New()},
	}

	tests := []struct {
		name      string
		security  security.EnforceSecurityUser
		userErr   error
		wantError error
	}{
		{name: "lists grants across tenants", security: allowOrganizationGrantSecurity{}},
		{name: "unknown user", security: allowOrganizationGrantSecurity{}, userErr: models.NotFoundError, wantError: models.NotFoundError},
		{name: "forbidden", security: denyListUserGrantsSecurity{}, wantError: models.ForbiddenError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executorFactory := &mocks.ExecutorFactory{}
			userRepository := &mocks.UserRepository{}
			grantRepository := &mocks.GrantRepository{}
			exec := &mocks.Executor{}

			executorFactory.On("NewExecutor").Return(exec)
			userRepository.On("UserById", mock.Anything, exec, userID).Return(models.User{UserId: models.UserId(userID)}, tt.userErr)
			grantRepository.On("ListOrganizationGrantsByUser", mock.Anything, exec, []string{userID}).
				Return(map[string][]models.Grant{userID: grants}, nil)

			usecase := UserUseCase{
				enforceUserSecurity: tt.security,
				executorFactory:     executorFactory,
				userRepository:      userRepository,
				grantRepository:     grantRepository,
			}
			got, err := usecase.ListUserGrants(context.Background(), userID)
			if tt.wantError != nil {
				require.ErrorIs(t, err, tt.wantError)
				grantRepository.AssertNotCalled(t, "ListOrganizationGrantsByUser", mock.Anything, mock.Anything, mock.Anything)
				return
			}
			require.NoError(t, err)
			require.Equal(t, grants, got)
		})
	}
}

func TestUserUseCaseListGrantsOfUsers(t *testing.T) {
	users := []models.User{{UserId: models.UserId(uuid.NewString())}, {UserId: models.UserId(uuid.NewString())}}
	userIDs := []string{string(users[0].UserId), string(users[1].UserId)}
	grantsByUser := map[string][]models.Grant{
		userIDs[0]: {{Role: models.VIEWER, TenantId: uuid.New(), OrganizationId: uuid.New()}},
	}

	t.Run("lists grants of every user in one query", func(t *testing.T) {
		executorFactory := &mocks.ExecutorFactory{}
		grantRepository := &mocks.GrantRepository{}
		exec := &mocks.Executor{}
		executorFactory.On("NewExecutor").Return(exec)
		grantRepository.On("ListOrganizationGrantsByUser", mock.Anything, exec, userIDs).Return(grantsByUser, nil).Once()

		usecase := UserUseCase{
			enforceUserSecurity: allowOrganizationGrantSecurity{},
			executorFactory:     executorFactory,
			grantRepository:     grantRepository,
		}
		got, err := usecase.ListGrantsOfUsers(context.Background(), users)
		require.NoError(t, err)
		require.Equal(t, grantsByUser, got)
		grantRepository.AssertExpectations(t)
	})

	t.Run("forbidden", func(t *testing.T) {
		grantRepository := &mocks.GrantRepository{}
		usecase := UserUseCase{
			enforceUserSecurity: denyListUserGrantsSecurity{},
			grantRepository:     grantRepository,
		}
		_, err := usecase.ListGrantsOfUsers(context.Background(), users)
		require.ErrorIs(t, err, models.ForbiddenError)
		grantRepository.AssertNotCalled(t, "ListOrganizationGrantsByUser", mock.Anything, mock.Anything, mock.Anything)
	})
}

var _ repositories.GrantRepository = (*mocks.GrantRepository)(nil)

func TestUserUseCaseAddUserWithoutOrganization(t *testing.T) {
	for _, role := range []models.Role{models.VIEWER, models.MARBLE_ADMIN} {
		t.Run(role.String(), func(t *testing.T) {
			userID := uuid.NewString()
			createUser := models.CreateUser{
				Email:     "new.user@example.com",
				Role:      role,
				FirstName: "New",
				LastName:  "User",
			}
			transactionFactory := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
			userRepository := &mocks.UserRepository{}
			organizationRepository := &mocks.OrganizationRepository{}

			transactionFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
			userRepository.On("CreateUser", mock.Anything, mock.Anything, createUser).Return(userID, nil)
			userRepository.On("UserById", mock.Anything, mock.Anything, userID).
				Return(models.User{UserId: models.UserId(userID), Email: createUser.Email, Role: role}, nil)

			usecase := UserUseCase{
				enforceUserSecurity:    allowOrganizationGrantSecurity{},
				transactionFactory:     transactionFactory,
				userRepository:         userRepository,
				organizationRepository: organizationRepository,
			}
			user, err := usecase.AddUser(context.Background(), createUser)
			require.NoError(t, err)
			require.Equal(t, uuid.Nil, user.OrganizationId)
			organizationRepository.AssertNotCalled(t, "GetOrganizationById", mock.Anything, mock.Anything, mock.Anything)
			userRepository.AssertExpectations(t)
		})
	}
}
