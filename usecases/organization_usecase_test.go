package usecases

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/security"
)

func TestUpdateOrganizationRestrictedSettings(t *testing.T) {
	tests := []struct {
		name    string
		roles   []models.Role
		body    string
		allowed bool
	}{
		{name: "admin environment", roles: []models.Role{models.ADMIN}, body: `{"environment":"staging"}`},
		{name: "admin unchanged environment", roles: []models.Role{models.ADMIN}, body: `{"environment":"production"}`},
		{name: "admin replay true", roles: []models.Role{models.ADMIN}, body: `{"sentry_replay_enabled":true}`},
		{name: "admin replay false", roles: []models.Role{models.ADMIN}, body: `{"sentry_replay_enabled":false}`},
		{name: "admin tenant admin both fields", roles: []models.Role{models.ADMIN, models.TENANT_ADMIN}, body: `{"environment":"demo","sentry_replay_enabled":true}`},
		{name: "admin mixed update", roles: []models.Role{models.ADMIN}, body: `{"environment":"staging","sanctions_limit":12}`},
		{name: "admin other field", roles: []models.Role{models.ADMIN}, body: `{"sanctions_limit":12}`, allowed: true},
		{name: "admin null fields", roles: []models.Role{models.ADMIN}, body: `{"environment":null,"sentry_replay_enabled":null,"sanctions_limit":12}`, allowed: true},
		{name: "marble admin environment", roles: []models.Role{models.MARBLE_ADMIN}, body: `{"environment":"staging"}`, allowed: true},
		{name: "marble admin replay true", roles: []models.Role{models.MARBLE_ADMIN}, body: `{"sentry_replay_enabled":true}`, allowed: true},
		{name: "marble admin replay false", roles: []models.Role{models.MARBLE_ADMIN}, body: `{"sentry_replay_enabled":false}`, allowed: true},
		{name: "marble admin both fields", roles: []models.Role{models.MARBLE_ADMIN}, body: `{"environment":"demo","sentry_replay_enabled":true}`, allowed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			orgID := uuid.New()
			org := models.Organization{Id: orgID, Environment: models.OrganizationEnvironmentProduction}
			creds := models.Credentials{OrganizationId: orgID, RoleBindings: models.NativeRoleBindings(tt.roles)}
			enforcer := &security.EnforceSecurityOrganizationImpl{
				EnforceSecurity: security.NewEnforceSecurity(creds),
				Credentials:     creds,
			}
			tx := &mocks.Transaction{}
			factory := &mocks.TransactionFactory{TxMock: tx}
			factory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
			repo := &mocks.OrganizationRepository{}
			repo.On("GetOrganizationById", mock.Anything, tx, orgID).Return(org, nil)
			if tt.allowed {
				repo.On("UpdateOrganization", mock.Anything, tx, orgID, mock.Anything).Return(nil).Once()
			}

			var body dto.UpdateOrganizationBodyDto
			require.NoError(t, json.Unmarshal([]byte(tt.body), &body))
			input, err := dto.AdaptUpdateOrganizationInput(body)
			require.NoError(t, err)
			uc := OrganizationUseCase{
				enforceSecurity:        enforcer,
				transactionFactory:     factory,
				organizationRepository: repo,
			}
			_, err = uc.UpdateOrganization(ctx, orgID, input)
			if tt.allowed {
				require.NoError(t, err)
				repo.AssertExpectations(t)
			} else {
				require.ErrorIs(t, err, models.ForbiddenError)
				repo.AssertNotCalled(t, "UpdateOrganization", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}
