package usecases

import (
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTenantAdministrationScope(t *testing.T) {
	tenantID := uuid.New()
	for _, tt := range []struct {
		name        string
		credentials models.Credentials
		allowed     bool
	}{
		{name: "tenant administrator", credentials: models.Credentials{Roles: []models.Role{models.TENANT_ADMIN}, TenantId: tenantID}, allowed: true},
		{name: "platform administrator", credentials: models.Credentials{Roles: []models.Role{models.MARBLE_ADMIN}}, allowed: true},
		{name: "organization administrator", credentials: models.Credentials{Roles: []models.Role{models.ADMIN}, TenantId: tenantID}},
		{name: "another tenant", credentials: models.Credentials{Roles: []models.Role{models.TENANT_ADMIN}, TenantId: uuid.New()}},
		{name: "inconsistent authentication tenant", credentials: models.Credentials{Roles: []models.Role{models.TENANT_ADMIN}, TenantId: tenantID, AuthTenantId: uuid.New()}},
		{name: "no tenant", credentials: models.Credentials{Roles: []models.Role{models.TENANT_ADMIN}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			users := &mocks.UserRepository{}
			grants := &mocks.GrantRepository{}
			factory := &mocks.ExecutorFactory{}
			transactions := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
			if tt.allowed {
				exec := &mocks.Executor{}
				factory.On("NewExecutor").Return(exec)
				users.On("ListUsersByTenant", mock.Anything, exec, tenantID).Return([]models.User{}, nil)
				grants.On("SetTenantAdministrator", mock.Anything, mock.Anything, tenantID, "other", true).Return(nil)
				transactions.On("Transaction", mock.Anything, mock.Anything).Return(nil)
			}
			uc := UserUseCase{credentials: tt.credentials, executorFactory: factory, transactionFactory: transactions, userRepository: users, grantRepository: grants}
			_, readErr := uc.ListUsersByTenant(t.Context(), tenantID)
			writeErr := uc.SetTenantAdministrator(t.Context(), tenantID, "other", true)
			if tt.allowed {
				require.NoError(t, readErr)
				require.NoError(t, writeErr)
				users.AssertExpectations(t)
				grants.AssertExpectations(t)
			} else {
				require.ErrorIs(t, readErr, models.ForbiddenError)
				require.ErrorIs(t, writeErr, models.ForbiddenError)
				users.AssertNotCalled(t, "ListUsersByTenant", mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}

func TestTenantAdministratorCannotChangeOwnGrant(t *testing.T) {
	tenantID := uuid.New()
	uc := UserUseCase{credentials: models.Credentials{Roles: []models.Role{models.TENANT_ADMIN}, TenantId: tenantID, ActorIdentity: models.Identity{UserId: "self"}}}
	for _, enabled := range []bool{true, false} {
		require.ErrorIs(t, uc.SetTenantAdministrator(t.Context(), tenantID, "self", enabled), models.ForbiddenError)
	}
}

func TestVisibleOrganizationGrants(t *testing.T) {
	tenant, other := uuid.New(), uuid.New()
	grants := []models.Grant{{TenantId: tenant, OrganizationId: uuid.New()}, {TenantId: other, OrganizationId: uuid.New()}}
	for _, tt := range []struct {
		name        string
		credentials models.Credentials
		want        int
	}{
		{name: "tenant administrator", credentials: models.Credentials{TenantId: tenant, Roles: []models.Role{models.TENANT_ADMIN}}, want: 1},
		{name: "platform administrator", credentials: models.Credentials{Roles: []models.Role{models.MARBLE_ADMIN}}, want: 2},
		{name: "missing tenant", credentials: models.Credentials{Roles: []models.Role{models.TENANT_ADMIN}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			uc := UserUseCase{credentials: tt.credentials}
			got := uc.visibleOrganizationGrants(append([]models.Grant(nil), grants...))
			require.Len(t, got, tt.want)
			if tt.want == 1 {
				require.Equal(t, tenant, got[0].TenantId)
			}
		})
	}
}

func TestTenantGrantListingsExcludeOtherTenants(t *testing.T) {
	tenant, other := uuid.New(), uuid.New()
	user := models.User{UserId: "shared"}
	local := models.Grant{TenantId: tenant, OrganizationId: uuid.New()}
	foreign := models.Grant{TenantId: other, OrganizationId: uuid.New()}
	exec := &mocks.Executor{}
	factory := &mocks.ExecutorFactory{}
	factory.On("NewExecutor").Return(exec)
	users := &mocks.UserRepository{}
	users.On("UserById", mock.Anything, exec, "shared").Return(user, nil)
	grants := &mocks.GrantRepository{}
	grants.On("ListOrganizationGrantsByUser", mock.Anything, exec, []string{"shared"}).
		Return(map[string][]models.Grant{"shared": {local, foreign}}, nil)
	uc := UserUseCase{credentials: models.Credentials{TenantId: tenant, Roles: []models.Role{models.TENANT_ADMIN}},
		enforceUserSecurity: allowOrganizationGrantSecurity{}, executorFactory: factory, userRepository: users, grantRepository: grants}
	got, err := uc.ListUserGrants(t.Context(), "shared")
	require.NoError(t, err)
	require.Equal(t, []models.Grant{local}, got)
	// The mock returns the same backing slice; provide a fresh result for the batch call.
	grants = &mocks.GrantRepository{}
	grants.On("ListOrganizationGrantsByUser", mock.Anything, exec, []string{"shared"}).
		Return(map[string][]models.Grant{"shared": {local, foreign}}, nil)
	uc.grantRepository = grants
	batch, err := uc.ListGrantsOfUsers(t.Context(), []models.User{user})
	require.NoError(t, err)
	require.Equal(t, []models.Grant{local}, batch["shared"])
}
