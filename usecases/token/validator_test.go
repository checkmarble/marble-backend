package token

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
)

func TestValidator_Validate_APIKey(t *testing.T) {
	key := "api_key"
	// hash of "api_key"
	keyHash, err := hex.DecodeString("2e9bc6c94a4cbdfe2a31d2df79103a5eb3702eaf5d7018d47a774e9540a8ec29")
	assert.NoError(t, err)

	apiKey := models.ApiKey{
		Id:             "api_key_id",
		OrganizationId: "organization_id",
		Role:           models.ADMIN,
	}

	organization := models.Organization{
		Id:   "organization_id",
		Name: "organization",
	}

	creds := models.Credentials{
		OrganizationId: "organization_id",
		Role:           models.ADMIN,
		ActorIdentity: models.Identity{
			ApiKeyName: "ApiKey Of organization",
		},
	}

	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		mockKeyAndOrganizationGetter := new(mocks.Database)
		mockKeyAndOrganizationGetter.On("GetApiKeyByHash", ctx, keyHash).
			Return(apiKey, nil)
		mockKeyAndOrganizationGetter.On("GetOrganizationByID", ctx, apiKey.OrganizationId).
			Return(organization, nil)

		v := Validator{
			getter: mockKeyAndOrganizationGetter,
		}

		credentials, err := v.Validate(ctx, "", key)
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

		_, err := v.Validate(ctx, "", key)
		assert.Error(t, err)
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

		_, err := v.Validate(ctx, "", key)
		assert.Error(t, err)
		mockKeyAndOrganizationGetter.AssertExpectations(t)
	})
}

func TestValidator_Validate_Token(t *testing.T) {
	token := "token"

	t.Run("nominal", func(t *testing.T) {
		creds := models.Credentials{
			OrganizationId: "organization_id",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				UserId: "user_id",
				Email:  "user@email.com",
			},
		}

		mockValidator := new(mocks.JWTEncoderValidator)
		mockValidator.On("ValidateMarbleToken", token).
			Return(creds, nil)

		v := Validator{
			validator: mockValidator,
		}

		credentials, err := v.Validate(context.Background(), token, "")
		assert.NoError(t, err)
		assert.Equal(t, creds, credentials)
		mockValidator.AssertExpectations(t)
	})

	t.Run("ValidateMarbleToken error", func(t *testing.T) {
		mockValidator := new(mocks.JWTEncoderValidator)
		mockValidator.On("ValidateMarbleToken", token).
			Return(models.Credentials{}, assert.AnError)

		v := Validator{
			validator: mockValidator,
		}

		_, err := v.Validate(context.Background(), token, "")
		assert.Error(t, err)
		mockValidator.AssertExpectations(t)
	})
}
