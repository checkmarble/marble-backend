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

var _ security.EnforceSecurityUser = allowOrganizationGrantSecurity{}

func TestUserUseCaseReplaceOrganizationGrant(t *testing.T) {
	userID := uuid.NewString()
	homeOrganizationID := uuid.New()
	targetOrganizationID := uuid.New()
	tenantID := uuid.New()

	tests := []struct {
		name       string
		homeTenant uuid.UUID
		userRole   models.Role
		homeOrg    uuid.UUID
		wantError  error
	}{
		{name: "same tenant", homeTenant: tenantID, homeOrg: homeOrganizationID},
		{name: "different tenant", homeTenant: uuid.New(), homeOrg: homeOrganizationID, wantError: models.NotFoundError},
		{name: "platform user", userRole: models.MARBLE_ADMIN},
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
			err := usecase.ReplaceOrganizationGrant(context.Background(), userID, targetOrganizationID, models.VIEWER)
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
	transactionFactory := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
	userRepository := &mocks.UserRepository{}
	grantRepository := &mocks.GrantRepository{}
	organizationRepository := &mocks.OrganizationRepository{}

	userRepository.On("UserById", mock.Anything, mock.Anything, userID).
		Return(models.User{Role: models.MARBLE_ADMIN}, nil)
	organizationRepository.On("GetOrganizationById", mock.Anything, mock.Anything, organizationID).
		Return(models.Organization{Id: organizationID}, nil)
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

	require.NoError(t, usecase.RevokeOrganizationGrant(context.Background(), userID, organizationID))
	grantRepository.AssertExpectations(t)
}

var _ repositories.GrantRepository = (*mocks.GrantRepository)(nil)
