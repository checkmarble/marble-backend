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
	}

	organization := models.Organization{
		Id:       utils.TextToUUID("organization_id"),
		Name:     "organization",
		TenantId: utils.TextToUUID("tenant_id"),
	}

	grants := []models.RoleBinding{
		{Role: models.ADMIN, OrgId: apiKey.OrganizationId, Permissions: models.ADMIN.Permissions()},
		{Role: models.TENANT_ADMIN, TenantId: organization.TenantId, Permissions: models.TENANT_ADMIN.Permissions()},
		{Role: models.VIEWER, OrgId: utils.TextToUUID("other_organization_id"), Permissions: models.VIEWER.Permissions()},
	}

	creds := models.Credentials{
		OrganizationId: utils.TextToUUID("organization_id"),
		RoleBindings:   grants[:2],
		Permissions:    models.ADMIN.Permissions(),
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
			Return(grants, nil)

		v := Validator{
			getter: mockKeyAndOrganizationGetter,
		}

		credentials, err := v.ValidateTokenOrKey(ctx, "", key)
		assert.NoError(t, err)
		assert.Equal(t, creds, credentials)
		mockKeyAndOrganizationGetter.AssertExpectations(t)
	})

	t.Run("no active grant", func(t *testing.T) {
		mockKeyAndOrganizationGetter := new(mocks.Database)
		mockKeyAndOrganizationGetter.On("GetApiKeyByHash", ctx, keyHash).
			Return(apiKey, nil)
		mockKeyAndOrganizationGetter.On("GetOrganizationByID", ctx, apiKey.OrganizationId).
			Return(organization, nil)
		mockKeyAndOrganizationGetter.On("ActiveGrantsForPrincipal", mock.Anything, "api_key", apiKey.Id).
			Return(grants[2:], nil)

		v := Validator{
			getter: mockKeyAndOrganizationGetter,
		}

		_, err := v.ValidateTokenOrKey(ctx, "", key)
		assert.ErrorIs(t, err, models.UnAuthorizedError)
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

	t.Run("GetOrganizationByID error", func(t *testing.T) {
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

func TestValidator_Validate_Token(t *testing.T) {
	token := "token"
	ctx := context.Background()

	organizationId := utils.TextToUUID("organization_id")
	tenantId := utils.TextToUUID("tenant_id")

	tokenCreds := models.Credentials{
		OrganizationId: organizationId,
		// Bindings embedded in the token are ignored in favor of current grants.
		RoleBindings: models.NativeRoleBindings([]models.Role{models.ADMIN}),
		ActorIdentity: models.Identity{
			UserId: "user_id",
			Email:  "user@email.com",
		},
	}

	grants := []models.RoleBinding{
		{Role: models.VIEWER, OrgId: organizationId, Permissions: models.VIEWER.Permissions()},
		{Role: models.MARBLE_ADMIN, Permissions: models.MARBLE_ADMIN.Permissions()},
	}

	t.Run("nominal", func(t *testing.T) {
		mockValidator := new(mocks.JWTEncoderValidator)
		mockValidator.On("ValidateMarbleToken", token).
			Return(tokenCreds, nil)
		mockGetter := new(mocks.Database)
		mockGetter.On("GetOrganizationByID", ctx, organizationId).
			Return(models.Organization{Id: organizationId, TenantId: tenantId}, nil)
		mockGetter.On("ActiveGrantsForPrincipal", ctx, "user", "user_id").
			Return(grants, nil)

		v := Validator{
			getter:    mockGetter,
			validator: mockValidator,
		}

		expected := tokenCreds
		expected.RoleBindings = grants[:1]
		expected.Permissions = models.VIEWER.Permissions()

		credentials, err := v.ValidateTokenOrKey(ctx, token, "")
		assert.NoError(t, err)
		assert.Equal(t, expected, credentials)
		mockValidator.AssertExpectations(t)
		mockGetter.AssertExpectations(t)
	})

	t.Run("tenant grant", func(t *testing.T) {
		mockValidator := new(mocks.JWTEncoderValidator)
		mockValidator.On("ValidateMarbleToken", token).
			Return(tokenCreds, nil)
		mockGetter := new(mocks.Database)
		mockGetter.On("GetOrganizationByID", ctx, organizationId).
			Return(models.Organization{Id: organizationId, TenantId: tenantId}, nil)
		mockGetter.On("ActiveGrantsForPrincipal", ctx, "user", "user_id").
			Return([]models.RoleBinding{{Role: models.TENANT_ADMIN, TenantId: tenantId}}, nil)

		v := Validator{
			getter:    mockGetter,
			validator: mockValidator,
		}

		credentials, err := v.ValidateTokenOrKey(ctx, token, "")
		assert.NoError(t, err)
		assert.True(t, credentials.HasRole(models.TENANT_ADMIN))
		mockGetter.AssertExpectations(t)
	})

	t.Run("revoked grants", func(t *testing.T) {
		mockValidator := new(mocks.JWTEncoderValidator)
		mockValidator.On("ValidateMarbleToken", token).
			Return(tokenCreds, nil)
		mockGetter := new(mocks.Database)
		mockGetter.On("GetOrganizationByID", ctx, organizationId).
			Return(models.Organization{Id: organizationId, TenantId: tenantId}, nil)
		mockGetter.On("ActiveGrantsForPrincipal", ctx, "user", "user_id").
			Return([]models.RoleBinding{}, nil)

		v := Validator{
			getter:    mockGetter,
			validator: mockValidator,
		}

		_, err := v.ValidateTokenOrKey(ctx, token, "")
		assert.ErrorIs(t, err, models.UnAuthorizedError)
	})

	t.Run("platform token", func(t *testing.T) {
		platformCreds := tokenCreds
		platformCreds.OrganizationId = [16]byte{}

		mockValidator := new(mocks.JWTEncoderValidator)
		mockValidator.On("ValidateMarbleToken", token).
			Return(platformCreds, nil)
		mockGetter := new(mocks.Database)
		mockGetter.On("ActiveGrantsForPrincipal", ctx, "user", "user_id").
			Return(grants, nil)

		v := Validator{
			getter:    mockGetter,
			validator: mockValidator,
		}

		credentials, err := v.ValidateTokenOrKey(ctx, token, "")
		assert.NoError(t, err)
		assert.Equal(t, grants[1:], credentials.RoleBindings)
		mockGetter.AssertExpectations(t)
	})

	t.Run("platform token without platform grants", func(t *testing.T) {
		platformCreds := tokenCreds
		platformCreds.OrganizationId = [16]byte{}

		mockValidator := new(mocks.JWTEncoderValidator)
		mockValidator.On("ValidateMarbleToken", token).
			Return(platformCreds, nil)
		mockGetter := new(mocks.Database)
		mockGetter.On("ActiveGrantsForPrincipal", ctx, "user", "user_id").
			Return([]models.RoleBinding{
				{Role: models.VIEWER, OrgId: organizationId, Permissions: models.VIEWER.Permissions()},
				{Role: models.TENANT_ADMIN, TenantId: tenantId},
			}, nil)

		v := Validator{
			getter:    mockGetter,
			validator: mockValidator,
		}

		// The user can still authenticate to select an organization, but holds
		// no role nor permission until then.
		credentials, err := v.ValidateTokenOrKey(ctx, token, "")
		assert.NoError(t, err)
		assert.Empty(t, credentials.RoleBindings)
		assert.Empty(t, credentials.Permissions)
		mockGetter.AssertExpectations(t)
	})

	t.Run("ValidateMarbleToken error", func(t *testing.T) {
		mockValidator := new(mocks.JWTEncoderValidator)
		mockValidator.On("ValidateMarbleToken", token).
			Return(models.Credentials{}, assert.AnError)

		v := Validator{
			validator: mockValidator,
		}

		_, err := v.ValidateTokenOrKey(ctx, token, "")
		assert.Error(t, err)
		mockValidator.AssertExpectations(t)
	})
}
