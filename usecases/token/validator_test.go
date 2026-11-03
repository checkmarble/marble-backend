package token

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

func TestValidator_Validate_APIKey(t *testing.T) {
	key := "api_key"
	// hash of "api_key"
	keyHash, err := hex.DecodeString("2e9bc6c94a4cbdfe2a31d2df79103a5eb3702eaf5d7018d47a774e9540a8ec29")
	assert.NoError(t, err)

	apiKey := models.ApiKey{
		Id:             "api_key_id",
		OrganizationId: utils.TextToUUID("organization_id"),
		Prefix:         "abc",
		Role:           models.ADMIN,
	}

	organization := models.Organization{
		Id:       utils.TextToUUID("organization_id"),
		Name:     "organization",
		TenantId: utils.TextToUUID("tenant_id"),
	}

	creds := models.Credentials{
		OrganizationId: utils.TextToUUID("organization_id"),
		TenantId:       organization.TenantId,
		Roles:          []models.Role{models.ADMIN, models.TENANT_ADMIN},
		ActorIdentity: models.Identity{
			ApiKeyId:   "api_key_id",
			ApiKeyName: "Api key abc*** of organization",
		},
	}

	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		mockKeyAndOrganizationGetter := new(mocks.Database)
		mockKeyAndOrganizationGetter.On("GetApiKeyByHash", ctx, keyHash).
			Return(apiKey, nil)
		mockKeyAndOrganizationGetter.On("GetOrganizationByID", ctx, apiKey.OrganizationId).
			Return(organization, nil)
		mockKeyAndOrganizationGetter.On("ActiveGrantsForPrincipal", mock.Anything, "api_key", apiKey.Id).
			Return([]models.Grant{
				{Role: models.ADMIN, OrganizationId: apiKey.OrganizationId},
				{Role: models.TENANT_ADMIN, TenantId: organization.TenantId},
			}, nil)

		v := Validator{
			getter: mockKeyAndOrganizationGetter,
		}

		credentials, err := v.ValidateTokenOrKey(ctx, "", key)
		assert.NoError(t, err)
		assert.Equal(t, creds, credentials)
		mockKeyAndOrganizationGetter.AssertExpectations(t)
	})

	t.Run("GetApiKeyByHash error", func(t *testing.T) {
		mockKeyAndOrganizationGetter := new(mocks.Database)
		mockKeyAndOrganizationGetter.On("GetApiKeyByHash", ctx, keyHash).
			Return(models.ApiKey{}, assert.AnError)

		v := Validator{
			getter: mockKeyAndOrganizationGetter,
		}

		_, err := v.ValidateTokenOrKey(ctx, "", key)
		assert.Error(t, err)
		mockKeyAndOrganizationGetter.AssertExpectations(t)
	})

	t.Run("rejects an API key without an applicable grant", func(t *testing.T) {
		mockKeyAndOrganizationGetter := new(mocks.Database)
		mockKeyAndOrganizationGetter.On("GetApiKeyByHash", ctx, keyHash).Return(apiKey, nil)
		mockKeyAndOrganizationGetter.On("GetOrganizationByID", ctx, apiKey.OrganizationId).Return(organization, nil)
		mockKeyAndOrganizationGetter.On("ActiveGrantsForPrincipal", mock.Anything, "api_key", apiKey.Id).
			Return([]models.Grant{{Role: models.ADMIN, OrganizationId: utils.TextToUUID("another_organization")}}, nil)

		v := Validator{getter: mockKeyAndOrganizationGetter}
		_, err := v.ValidateTokenOrKey(ctx, "", key)

		assert.ErrorIs(t, err, models.UnAuthorizedError)
		mockKeyAndOrganizationGetter.AssertExpectations(t)
	})

	t.Run("nominal", func(t *testing.T) {
		mockKeyAndOrganizationGetter := new(mocks.Database)
		mockKeyAndOrganizationGetter.On("GetApiKeyByHash", ctx, keyHash).
			Return(apiKey, nil)
		mockKeyAndOrganizationGetter.On("GetOrganizationByID", ctx, apiKey.OrganizationId).
			Return(models.Organization{}, assert.AnError)

		v := Validator{
			getter: mockKeyAndOrganizationGetter,
		}

		_, err := v.ValidateTokenOrKey(ctx, "", key)
		assert.Error(t, err)
		mockKeyAndOrganizationGetter.AssertExpectations(t)
	})
}

func TestValidatorRejectsStaleAndCrossTenantPrivileges(t *testing.T) {
	tenantID, otherTenant, orgID := uuid.New(), uuid.New(), uuid.New()
	for _, tt := range []struct {
		name        string
		credentials models.Credentials
		grants      []models.Grant
		allowed     bool
		roles       []models.Role
	}{
		{name: "revoked tenant administrator", credentials: models.Credentials{TenantId: tenantID, Roles: []models.Role{models.TENANT_ADMIN}}},
		{name: "merged identity cannot use old tenant", credentials: models.Credentials{TenantId: tenantID, Roles: []models.Role{models.TENANT_ADMIN}}, grants: []models.Grant{{TenantId: otherTenant, Role: models.TENANT_ADMIN}}},
		{name: "organization grant never implies tenant administration", credentials: models.Credentials{TenantId: tenantID, Roles: []models.Role{models.TENANT_ADMIN}}, grants: []models.Grant{{OrganizationId: orgID, Role: models.ADMIN}}},
		{name: "revoked platform administrator", credentials: models.Credentials{Roles: []models.Role{models.MARBLE_ADMIN}}, grants: []models.Grant{{TenantId: tenantID, Role: models.TENANT_ADMIN}}},
		{name: "tenant authentication excludes platform grants", credentials: models.Credentials{TenantId: tenantID, AuthTenantId: tenantID, Roles: []models.Role{models.TENANT_ADMIN}}, grants: []models.Grant{{TenantId: tenantID, Role: models.TENANT_ADMIN}, {Role: models.MARBLE_ADMIN}}, allowed: true, roles: []models.Role{models.TENANT_ADMIN}},
		{name: "new administrator grant does not elevate an existing token", credentials: models.Credentials{TenantId: tenantID}, grants: []models.Grant{{TenantId: tenantID, Role: models.TENANT_ADMIN}}, allowed: true, roles: []models.Role{}},
		{name: "organization role downgrade invalidates the old token", credentials: models.Credentials{TenantId: tenantID, OrganizationId: orgID, Roles: []models.Role{models.ADMIN}}, grants: []models.Grant{{OrganizationId: orgID, Role: models.VIEWER}}},
		{name: "tenant administrator requires direct access to organization", credentials: models.Credentials{TenantId: tenantID, OrganizationId: orgID, Roles: []models.Role{models.TENANT_ADMIN}}, grants: []models.Grant{{TenantId: tenantID, Role: models.TENANT_ADMIN}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			credentials := tt.credentials
			credentials.ActorIdentity.UserId = "user"
			getter := &mocks.Database{}
			getter.On("GetUserByID", mock.Anything, "user").Return(models.User{TenantId: credentials.TenantId}, nil)
			getter.On("ActiveGrantsForPrincipal", mock.Anything, "user", "user").Return(tt.grants, nil)
			validator := Validator{getter: getter}
			result, err := validator.revalidateUserGrants(t.Context(), credentials)
			if tt.allowed {
				assert.NoError(t, err)
				assert.Equal(t, tt.roles, result.Roles)
			} else {
				assert.ErrorIs(t, err, models.UnAuthorizedError)
			}
		})
	}
}

func TestValidatorRejectsDeletedOrMovedIdentityBeforeReadingGrants(t *testing.T) {
	tenantID := uuid.New()
	for _, tt := range []struct {
		name string
		user models.User
		err  error
	}{
		{name: "deleted identity", err: models.NotFoundError},
		{name: "identity moved to another tenant", user: models.User{TenantId: uuid.New()}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			getter := &mocks.Database{}
			getter.On("GetUserByID", mock.Anything, "user").Return(tt.user, tt.err)
			validator := Validator{getter: getter}
			_, err := validator.revalidateUserGrants(t.Context(), models.Credentials{ActorIdentity: models.Identity{UserId: "user"}, TenantId: tenantID, AuthTenantId: tenantID})
			assert.ErrorIs(t, err, models.UnAuthorizedError)
			getter.AssertNotCalled(t, "ActiveGrantsForPrincipal", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestValidatorPreservesIdentityDatabaseErrors(t *testing.T) {
	getter := &mocks.Database{}
	getter.On("GetUserByID", mock.Anything, "user").Return(models.User{}, assert.AnError)
	validator := Validator{getter: getter}
	_, err := validator.revalidateUserGrants(t.Context(), models.Credentials{ActorIdentity: models.Identity{UserId: "user"}})
	assert.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, models.UnAuthorizedError)
}

func TestValidatorResolvesTenantAdministratorWithoutOrganization(t *testing.T) {
	tenantID := uuid.New()
	for _, tt := range []struct {
		name       string
		authTenant uuid.UUID
		grant      uuid.UUID
		allowed    bool
	}{
		{name: "tenant OIDC", authTenant: tenantID, grant: tenantID, allowed: true},
		{name: "global identity", grant: tenantID, allowed: true},
		{name: "different authentication tenant", authTenant: uuid.New(), grant: tenantID},
		{name: "different grant tenant", grant: uuid.New()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			credentials := models.Credentials{ActorIdentity: models.Identity{UserId: "user"}, AuthTenantId: tt.authTenant, Roles: []models.Role{models.TENANT_ADMIN}}
			jwt := new(mocks.JWTEncoderValidator)
			jwt.On("ValidateMarbleToken", "token").Return(credentials, nil)
			getter := new(mocks.Database)
			getter.On("GetUserByID", mock.Anything, "user").Return(models.User{TenantId: tenantID}, nil)
			if tt.authTenant == uuid.Nil || tt.authTenant == tenantID {
				getter.On("ActiveGrantsForPrincipal", mock.Anything, "user", "user").Return([]models.Grant{{TenantId: tt.grant, Role: models.TENANT_ADMIN}}, nil)
			}
			result, err := (&Validator{getter: getter, validator: jwt}).ValidateTokenOrKey(t.Context(), "token", "")
			if tt.allowed {
				assert.NoError(t, err)
				assert.Equal(t, tenantID, result.TenantId)
				assert.Equal(t, []models.Role{models.TENANT_ADMIN}, result.Roles)
			} else {
				assert.ErrorIs(t, err, models.UnAuthorizedError)
			}
		})
	}
}

func TestValidator_Validate_Token(t *testing.T) {
	token := "token"
	organization := models.Organization{
		Id:       utils.TextToUUID("organization_id"),
		TenantId: utils.TextToUUID("tenant_id"),
	}

	t.Run("nominal", func(t *testing.T) {
		creds := models.Credentials{
			OrganizationId: utils.TextToUUID("organization_id"),
			Roles:          []models.Role{models.ADMIN},
			ActorIdentity: models.Identity{
				UserId: "user_id",
				Email:  "user@email.com",
			},
		}

		mockValidator := new(mocks.JWTEncoderValidator)
		mockValidator.On("ValidateMarbleToken", token).
			Return(creds, nil)
		mockGetter := new(mocks.Database)
		mockGetter.On("GetOrganizationByID", mock.Anything, creds.OrganizationId).
			Return(organization, nil)

		v := Validator{
			validator: mockValidator,
			getter:    mockGetter,
		}

		credentials, err := v.ValidateTokenOrKey(context.Background(), token, "")
		assert.NoError(t, err)
		assert.Equal(t, organization.TenantId, credentials.TenantId)
		mockValidator.AssertExpectations(t)
		mockGetter.AssertExpectations(t)
	})

	t.Run("platform token does not resolve a tenant", func(t *testing.T) {
		creds := models.Credentials{Roles: []models.Role{models.MARBLE_ADMIN}}
		mockValidator := new(mocks.JWTEncoderValidator)
		mockValidator.On("ValidateMarbleToken", token).Return(creds, nil)

		v := Validator{validator: mockValidator}
		credentials, err := v.ValidateTokenOrKey(context.Background(), token, "")

		assert.NoError(t, err)
		assert.Equal(t, creds, credentials)
		mockValidator.AssertExpectations(t)
	})

	t.Run("tenant-bound token rejects an organization from another tenant", func(t *testing.T) {
		creds := models.Credentials{
			OrganizationId: organization.Id,
			AuthTenantId:   utils.TextToUUID("another_tenant"),
		}
		mockValidator := new(mocks.JWTEncoderValidator)
		mockValidator.On("ValidateMarbleToken", token).Return(creds, nil)
		mockGetter := new(mocks.Database)
		mockGetter.On("GetOrganizationByID", mock.Anything, creds.OrganizationId).Return(organization, nil)

		v := Validator{validator: mockValidator, getter: mockGetter}
		_, err := v.ValidateTokenOrKey(context.Background(), token, "")

		assert.ErrorIs(t, err, models.UnAuthorizedError)
		mockValidator.AssertExpectations(t)
		mockGetter.AssertExpectations(t)
	})

	t.Run("ValidateMarbleToken error", func(t *testing.T) {
		mockValidator := new(mocks.JWTEncoderValidator)
		mockValidator.On("ValidateMarbleToken", token).
			Return(models.Credentials{}, assert.AnError)

		v := Validator{
			validator: mockValidator,
		}

		_, err := v.ValidateTokenOrKey(context.Background(), token, "")
		assert.Error(t, err)
		mockValidator.AssertExpectations(t)
	})
}
