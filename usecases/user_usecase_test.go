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
func (allowOrganizationGrantSecurity) ManageRoles() error { return nil }

var _ security.EnforceSecurityUser = allowOrganizationGrantSecurity{}

func TestUserUseCaseReplaceOrganizationGrant(t *testing.T) {
	userID := uuid.NewString()
	homeOrganizationID := uuid.New()
	targetOrganizationID := uuid.New()
	tenantID := uuid.New()

	tests := []struct {
		name       string
		homeTenant uuid.UUID
		userRoles  []models.Role
		homeOrg    uuid.UUID
		wantError  error
	}{
		{name: "same tenant", homeTenant: tenantID, homeOrg: homeOrganizationID},
		{name: "different tenant", homeTenant: uuid.New(), homeOrg: homeOrganizationID, wantError: models.NotFoundError},
		{name: "platform user", userRoles: []models.Role{models.MARBLE_ADMIN}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transactionFactory := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
			userRepository := &mocks.UserRepository{}
			organizationRepository := &mocks.OrganizationRepository{}

			userRepository.On("UserById", mock.Anything, mock.Anything, userID).Return(models.User{
				OrganizationId: tt.homeOrg,
				RoleBindings:   models.NativeRoleBindings(tt.userRoles),
			}, nil)
			organizationRepository.On("GetOrganizationById", mock.Anything, mock.Anything, targetOrganizationID).Return(models.Organization{Id: targetOrganizationID, TenantId: tenantID}, nil)
			if tt.homeOrg != uuid.Nil {
				organizationRepository.On("GetOrganizationById", mock.Anything, mock.Anything, tt.homeOrg).Return(models.Organization{Id: tt.homeOrg, TenantId: tt.homeTenant}, nil)
			}
			transactionFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
			if tt.wantError == nil {
				userRepository.On("ReplaceUserOrganizationRoleBindings", mock.Anything, mock.Anything, targetOrganizationID, userID,
					[]models.RoleBinding{models.NewNativeRoleBinding(models.VIEWER), models.NewNativeRoleBinding(models.ANALYST)}).
					Return(nil)
			}

			usecase := UserUseCase{
				enforceUserSecurity:    allowOrganizationGrantSecurity{},
				executorFactory:        newExecutorFactory(),
				transactionFactory:     transactionFactory,
				userRepository:         userRepository,
				organizationRepository: organizationRepository,
			}
			err := usecase.ReplaceOrganizationGrant(context.Background(), userID, targetOrganizationID,
				[]models.RoleBinding{{Role: models.VIEWER}, {Role: models.ANALYST}})
			if tt.wantError != nil {
				require.ErrorIs(t, err, tt.wantError)
				userRepository.AssertNotCalled(t, "ReplaceUserOrganizationRoleBindings", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				return
			}
			require.NoError(t, err)
			userRepository.AssertExpectations(t)
			transactionFactory.AssertExpectations(t)
		})
	}
}

func TestUserUseCaseReplaceOrganizationGrantRefusesPlatformRoles(t *testing.T) {
	userRepository := &mocks.UserRepository{}
	usecase := UserUseCase{
		enforceUserSecurity: allowOrganizationGrantSecurity{},
		executorFactory:     newExecutorFactory(),
		userRepository:      userRepository,
	}

	err := usecase.ReplaceOrganizationGrant(context.Background(), uuid.NewString(), uuid.New(),
		[]models.RoleBinding{{Role: models.MARBLE_ADMIN}})

	require.ErrorIs(t, err, models.BadParameterError)
	userRepository.AssertNotCalled(t, "ReplaceUserOrganizationRoleBindings", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestUserUseCaseRevokePlatformUserGrant(t *testing.T) {
	userID := uuid.NewString()
	organizationID := uuid.New()
	transactionFactory := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
	userRepository := &mocks.UserRepository{}
	organizationRepository := &mocks.OrganizationRepository{}

	userRepository.On("UserById", mock.Anything, mock.Anything, userID).
		Return(models.User{RoleBindings: models.NativeRoleBindings([]models.Role{models.MARBLE_ADMIN})}, nil)
	organizationRepository.On("GetOrganizationById", mock.Anything, mock.Anything, organizationID).
		Return(models.Organization{Id: organizationID}, nil)
	transactionFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
	// Revoking replaces the user's bindings in the organization with none.
	userRepository.On("ReplaceUserOrganizationRoleBindings", mock.Anything, mock.Anything, organizationID, userID,
		mock.MatchedBy(func(bindings []models.RoleBinding) bool { return len(bindings) == 0 })).Return(nil)

	usecase := UserUseCase{
		enforceUserSecurity:    allowOrganizationGrantSecurity{},
		executorFactory:        newExecutorFactory(),
		transactionFactory:     transactionFactory,
		userRepository:         userRepository,
		organizationRepository: organizationRepository,
	}

	require.NoError(t, usecase.RevokeOrganizationGrant(context.Background(), userID, organizationID))
	userRepository.AssertExpectations(t)
}

func TestUserUseCaseReplaceOrganizationGrantResolvesCustomRolesInTargetOrganization(t *testing.T) {
	userID := uuid.NewString()
	homeOrganizationID := uuid.New()
	targetOrganizationID := uuid.New()
	tenantID := uuid.New()
	customRoleId := uuid.New()

	transactionFactory := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
	userRepository := &mocks.UserRepository{}
	organizationRepository := &mocks.OrganizationRepository{}

	// The slug is looked up in the organization the binding is granted in, not
	// in the user's home organization.
	userRepository.On("GetRoleBySlug", mock.Anything, mock.Anything, targetOrganizationID, models.Role("org/reviewer")).
		Return(models.RbacRole{Id: customRoleId, Permissions: []models.Permission{models.CASE_READ_WRITE}}, nil)
	userRepository.On("UserById", mock.Anything, mock.Anything, userID).
		Return(models.User{OrganizationId: homeOrganizationID}, nil)
	organizationRepository.On("GetOrganizationById", mock.Anything, mock.Anything, targetOrganizationID).
		Return(models.Organization{Id: targetOrganizationID, TenantId: tenantID}, nil)
	organizationRepository.On("GetOrganizationById", mock.Anything, mock.Anything, homeOrganizationID).
		Return(models.Organization{Id: homeOrganizationID, TenantId: tenantID}, nil)
	transactionFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
	userRepository.On("ReplaceUserOrganizationRoleBindings", mock.Anything, mock.Anything, targetOrganizationID, userID,
		[]models.RoleBinding{{
			Role:         "org/reviewer",
			CustomRoleId: &customRoleId,
			Permissions:  []models.Permission{models.CASE_READ_WRITE},
		}}).Return(nil)

	usecase := UserUseCase{
		enforceUserSecurity:    allowOrganizationGrantSecurity{},
		executorFactory:        newExecutorFactory(),
		transactionFactory:     transactionFactory,
		userRepository:         userRepository,
		organizationRepository: organizationRepository,
	}

	require.NoError(t, usecase.ReplaceOrganizationGrant(context.Background(), userID, targetOrganizationID,
		[]models.RoleBinding{{Role: "org/reviewer"}}))
	userRepository.AssertExpectations(t)
}

func newExecutorFactory() *mocks.ExecutorFactory {
	executorFactory := new(mocks.ExecutorFactory)
	executorFactory.On("NewExecutor").Return(new(mocks.Executor))

	return executorFactory
}

var _ repositories.GrantRepository = (*mocks.GrantRepository)(nil)
