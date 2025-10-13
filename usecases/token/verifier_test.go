package token

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/clock"
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGenerator_VerifyToken_APIKey(t *testing.T) {
	key := "api_key"
	// hash of "api_key"
	keyHash, err := hex.DecodeString("2e9bc6c94a4cbdfe2a31d2df79103a5eb3702eaf5d7018d47a774e9540a8ec29")
	assert.NoError(t, err)

	apiKey := models.ApiKey{
		Id:             "api_key_id",
		OrganizationId: "organization_id",
		Prefix:         "abc",
		Role:           models.ADMIN,
		DisplayString:  "Api key abc*** of organization",
	}

	organization := models.Organization{
		Id:   "organization_id",
		Name: "organization",
	}

	ctx := t.Context()

	idpTokenVerifier := mocks.NewStaticIdpTokenVerifier(infra.MockFirebaseIssuer, models.FirebaseIdentity{Issuer: infra.MockFirebaseIssuer, Email: "user@email.com"})

	t.Run("nominal", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByHash", ctx, keyHash).
			Return(apiKey, nil)
		mockRepository.On("GetOrganizationByID", ctx, "organization_id").
			Return(organization, nil)

		verifier := auth.NewVerifier(auth.TokenProviderFirebase, idpTokenVerifier, mockRepository)
		intoCreds, _, err := verifier.Verify(ctx, auth.Credentials{Type: auth.CredentialsApiKey, Value: key})

		assert.NoError(t, err)
		assert.Equal(t, apiKey.Id, intoCreds.IntoCredentials().ActorIdentity.ApiKeyId)
		assert.Equal(t, apiKey.OrganizationId, intoCreds.IntoCredentials().OrganizationId)
	})

	t.Run("GetApiKeyByHash error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByHash", ctx, keyHash).
			Return(models.ApiKey{}, assert.AnError)

		verifier := auth.NewVerifier(auth.TokenProviderFirebase, idpTokenVerifier, mockRepository)
		_, _, err := verifier.Verify(ctx, auth.Credentials{Type: auth.CredentialsApiKey, Value: key})

		assert.Error(t, err)

		mockRepository.AssertExpectations(t)
	})

	t.Run("GetOrganizationByID error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByHash", ctx, keyHash).
			Return(apiKey, nil)
		mockRepository.On("GetOrganizationByID", ctx, "organization_id").
			Return(models.Organization{}, assert.AnError)

		verifier := auth.NewVerifier(auth.TokenProviderFirebase, idpTokenVerifier, mockRepository)
		_, _, err := verifier.Verify(ctx, auth.Credentials{Type: auth.CredentialsApiKey, Value: key})

		assert.Error(t, err)

		mockRepository.AssertExpectations(t)
	})
}

func TestGenerator_VerifyToken_FirebaseToken(t *testing.T) {
	firebaseToken := auth.Credentials{Type: auth.CredentialsBearer, Value: "firebaseToken"}
	firebaseIdentity := models.FirebaseIdentity{
		Issuer: infra.MockFirebaseIssuer,
		Email:  "user@email.com",
	}
	token := "token"
	now := time.Now()

	user := models.User{
		UserId:         "user_id",
		Email:          "user@email.com",
		Role:           models.ADMIN,
		OrganizationId: "organization_id",
	}

	idpTokenVerifier := mocks.NewStaticIdpTokenVerifier(infra.MockFirebaseIssuer, firebaseIdentity)

	t.Run("nominal", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetOrganizationByID", mock.Anything, "organization_id").
			Return(models.Organization{}, nil)
		mockRepository.On("UserByEmail", mock.Anything, firebaseIdentity.Email).
			Return(user, nil)
		mockRepository.On("UpdateUser", mock.Anything, user, models.IdentityUpdatableClaims{}).
			Return(user, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", infra.MockFirebaseIssuer, mock.Anything, models.Credentials{
			OrganizationId: "organization_id",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				UserId: user.UserId,
				Email:  user.Email,
			},
		}).
			Return(token, nil)

		verifier := auth.NewVerifier(auth.TokenProviderFirebase, idpTokenVerifier, mockRepository)
		generator := auth.NewGenerator(
			mockRepository,
			mockEncoder,
			60*time.Second,
			clock.NewMock(now),
		)

		tokenHandler := auth.NewTokenHandler(mocks.NewStaticTokenExtractor(firebaseToken), verifier, generator)
		receivedToken, err := tokenHandler.GetToken(context.Background(), nil)

		assert.NoError(t, err)
		assert.Equal(t, token, receivedToken.Value)
		assert.Equal(t, now.Add(60*time.Second), receivedToken.Expiration)
		mockRepository.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})

	t.Run("VerifyFirebaseToken error", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("Verify", mock.Anything, firebaseToken).
			Return(nil, nil, assert.AnError)

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
		mockRepository := new(mocks.Database)
		mockRepository.On("UserByEmail", mock.Anything, firebaseIdentity.Email).
			Return(models.User{}, assert.AnError)

		verifier := auth.NewVerifier(auth.TokenProviderFirebase, idpTokenVerifier, mockRepository)
		generator := auth.NewGenerator(
			mockRepository,
			nil,
			60*time.Second,
			clock.NewMock(now),
		)

		tokenHandler := auth.NewTokenHandler(mocks.NewStaticTokenExtractor(firebaseToken), verifier, generator)
		_, err := tokenHandler.GetToken(context.Background(), nil)

		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
	})

}
