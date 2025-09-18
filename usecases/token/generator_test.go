package token

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/clock"
	"github.com/checkmarble/marble-backend/usecases/auth"
)

func TestGenerator_GenerateToken_APIKey(t *testing.T) {
	key := "api_key"
	// hash of "api_key"
	keyHash, err := hex.DecodeString("2e9bc6c94a4cbdfe2a31d2df79103a5eb3702eaf5d7018d47a774e9540a8ec29")
	assert.NoError(t, err)

	apiKey := models.ApiKey{
		Id:             "api_key_id",
		OrganizationId: "organization_id",
		Prefix:         "abc",
		Role:           models.ADMIN,
	}

	organization := models.Organization{
		Id:   "organization_id",
		Name: "organization",
	}

	token := "token"
	now := time.Now()

	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByHash", ctx, keyHash).
			Return(apiKey, nil)
		mockRepository.On("GetOrganizationByID", ctx, "organization_id").
			Return(organization, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", "", mock.Anything, models.Credentials{
			OrganizationId: "organization_id",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				ApiKeyId:   "api_key_id",
				ApiKeyName: "Api key abc*** of organization",
			},
		}).
			Return(token, nil)

		generator := auth.NewGenerator(
			mockRepository,
			mockEncoder,
			60*time.Second,
			clock.NewMock(now),
		)

		creds, err := generator.GenerateToken(ctx, auth.Credentials{Type: auth.CredentialsApiKey, Value: key}, models.FirebaseIdentity{})
		assert.NoError(t, err)
		assert.Equal(t, token, creds.Value)
		assert.Equal(t, now.Add(60*time.Second), creds.Expiration)

		mockRepository.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})

	t.Run("GetApiKeyByHash error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByHash", ctx, keyHash).
			Return(models.ApiKey{}, assert.AnError)

		generator := auth.NewGenerator(
			mockRepository,
			nil,
			time.Hour,
			clock.New(),
		)

		_, err := generator.GenerateToken(ctx, auth.Credentials{Type: auth.CredentialsApiKey, Value: key}, models.FirebaseIdentity{})
		assert.Error(t, err)

		mockRepository.AssertExpectations(t)
	})

	t.Run("GetOrganizationByID error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByHash", ctx, keyHash).
			Return(apiKey, nil)
		mockRepository.On("GetOrganizationByID", ctx, "organization_id").
			Return(models.Organization{}, assert.AnError)

		generator := auth.NewGenerator(
			mockRepository,
			nil,
			time.Hour,
			clock.New(),
		)

		_, err := generator.GenerateToken(ctx, auth.Credentials{Type: auth.CredentialsApiKey, Value: key}, models.FirebaseIdentity{})
		assert.Error(t, err)

		mockRepository.AssertExpectations(t)
	})

	t.Run("EncodeMarbleToken error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByHash", ctx, keyHash).
			Return(apiKey, nil)
		mockRepository.On("GetOrganizationByID", ctx, "organization_id").
			Return(organization, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", "", mock.Anything, models.Credentials{
			OrganizationId: "organization_id",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				ApiKeyId:   "api_key_id",
				ApiKeyName: "Api key abc*** of organization",
			},
		}).
			Return(token, nil)

		generator := auth.NewGenerator(
			mockRepository,
			mockEncoder,
			60*time.Second,
			clock.NewMock(now),
		)

		receivedToken, err := generator.GenerateToken(ctx, auth.Credentials{Type: auth.CredentialsApiKey, Value: key}, models.FirebaseIdentity{})
		assert.NoError(t, err)
		assert.Equal(t, token, receivedToken.Value)
		assert.Equal(t, now.Add(60*time.Second), receivedToken.Expiration)

		mockRepository.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})
}

func TestGenerator_GenerateToken_FirebaseToken(t *testing.T) {
	firebaseToken := auth.Credentials{Type: auth.CredentialsBearer, Value: "firebaseToken"}
	firebaseIdentity := models.FirebaseIdentity{
		Email: "user@email.com",
	}
	token := "token"
	now := time.Now()

	user := models.User{
		UserId:         "user_id",
		Email:          "user@email.com",
		Role:           models.ADMIN,
		OrganizationId: "organization_id",
	}

	t.Run("nominal", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("Verify", mock.Anything, firebaseToken).
			Return(firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("UserByEmail", mock.Anything, firebaseIdentity.Email).
			Return(user, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, "organization_id").
			Return(models.Organization{}, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", "", mock.Anything, models.Credentials{
			OrganizationId: "organization_id",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				UserId: user.UserId,
				Email:  user.Email,
			},
		}).
			Return(token, nil)

		generator := auth.NewGenerator(
			mockRepository,
			mockEncoder,
			60*time.Second,
			clock.NewMock(now),
		)

		tokenHandler := auth.NewTokenHandler(mocks.NewStaticTokenExtractor(firebaseToken), mockVerifier, generator)
		receivedToken, err := tokenHandler.GetToken(context.Background(), nil)

		assert.NoError(t, err)
		assert.Equal(t, token, receivedToken.Value)
		assert.Equal(t, now.Add(60*time.Second), receivedToken.Expiration)
		mockRepository.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})

	t.Run("nominal first connection", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("Verify", mock.Anything, firebaseToken).
			Return(firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("UserByEmail", mock.Anything, firebaseIdentity.Email).
			Return(user, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, "organization_id").
			Return(models.Organization{}, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", "", mock.Anything, models.Credentials{
			OrganizationId: "organization_id",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				UserId: user.UserId,
				Email:  user.Email,
			},
		}).
			Return(token, nil)

		generator := auth.NewGenerator(
			mockRepository,
			mockEncoder,
			60*time.Second,
			clock.NewMock(now),
		)

		tokenHandler := auth.NewTokenHandler(mocks.NewStaticTokenExtractor(firebaseToken), mockVerifier, generator)
		receivedToken, err := tokenHandler.GetToken(context.Background(), nil)

		assert.NoError(t, err)
		assert.Equal(t, token, receivedToken.Value)
		assert.Equal(t, now.Add(60*time.Second), receivedToken.Expiration)
		mockRepository.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})

	t.Run("VerifyFirebaseToken error", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("Verify", mock.Anything, firebaseToken).
			Return(models.FirebaseIdentity{}, assert.AnError)

		generator := auth.NewGenerator(
			nil,
			nil,
			60*time.Second,
			clock.NewMock(now),
		)

		tokenHandler := auth.NewTokenHandler(mocks.NewStaticTokenExtractor(firebaseToken), mockVerifier, generator)
		_, err := tokenHandler.GetToken(context.Background(), nil)

		assert.Error(t, err)
		mockVerifier.AssertExpectations(t)
	})

	t.Run("UserByEmail error", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("Verify", mock.Anything, firebaseToken).
			Return(firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("UserByEmail", mock.Anything, firebaseIdentity.Email).
			Return(models.User{}, assert.AnError)

		generator := auth.NewGenerator(
			mockRepository,
			nil,
			60*time.Second,
			clock.NewMock(now),
		)

		tokenHandler := auth.NewTokenHandler(mocks.NewStaticTokenExtractor(firebaseToken), mockVerifier, generator)
		_, err := tokenHandler.GetToken(context.Background(), nil)

		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
	})

	t.Run("EncodeMarbleToken error", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("Verify", mock.Anything, firebaseToken).
			Return(firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("UserByEmail", mock.Anything, firebaseIdentity.Email).
			Return(user, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, "organization_id").
			Return(models.Organization{}, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", "", mock.Anything, models.Credentials{
			OrganizationId: "organization_id",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				UserId: user.UserId,
				Email:  user.Email,
			},
		}).
			Return("", assert.AnError)

		generator := auth.NewGenerator(
			mockRepository,
			mockEncoder,
			60*time.Second,
			clock.NewMock(now),
		)

		tokenHandler := auth.NewTokenHandler(mocks.NewStaticTokenExtractor(firebaseToken), mockVerifier, generator)
		_, err := tokenHandler.GetToken(context.Background(), nil)

		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})
}
