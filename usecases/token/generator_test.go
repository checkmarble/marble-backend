package token

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/clock"
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/checkmarble/marble-backend/utils"
)

func TestGenerator_GenerateToken_APIKey(t *testing.T) {
	key := "api_key"

	apiKey := models.ApiKey{
		Id:             "api_key_id",
		OrganizationId: utils.TextToUUID("organization_id"),
		Prefix:         "abc",
		Role:           models.ADMIN,
		DisplayString:  "Api key abc*** of organization",
	}

	token := "token"
	now := time.Now()

	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		mockRepository := new(mocks.Database)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", "", mock.Anything, models.Credentials{
			OrganizationId: utils.TextToUUID("organization_id"),
			Role:           models.ADMIN,
			Roles:          []models.Role{models.ADMIN},
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

		mockRepository.On("ActiveGrantsForPrincipal", mock.Anything, "api_key", apiKey.Id).
			Return([]models.Grant{{Role: models.ADMIN, OrganizationId: apiKey.OrganizationId}}, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, apiKey.OrganizationId).Return(models.Organization{}, nil)
		creds, err := generator.GenerateToken(ctx, auth.Credentials{
			Type: auth.CredentialsApiKey, Value: key,
		}, apiKey, models.FirebaseIdentity{}, apiKey.OrganizationId)
		assert.NoError(t, err)
		assert.Equal(t, token, creds.Value)
		assert.Equal(t, now.Add(60*time.Second), creds.Expiration)

		mockRepository.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})

	t.Run("EncodeMarbleToken error", func(t *testing.T) {
		mockRepository := new(mocks.Database)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", "", mock.Anything, models.Credentials{
			OrganizationId: utils.TextToUUID("organization_id"),
			Role:           models.ADMIN,
			Roles:          []models.Role{models.ADMIN},
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

		mockRepository.On("ActiveGrantsForPrincipal", mock.Anything, "api_key", apiKey.Id).
			Return([]models.Grant{{Role: models.ADMIN, OrganizationId: apiKey.OrganizationId}}, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, apiKey.OrganizationId).Return(models.Organization{}, nil)
		receivedToken, err := generator.GenerateToken(ctx, auth.Credentials{
			Type: auth.CredentialsApiKey, Value: key,
		}, apiKey, models.FirebaseIdentity{}, apiKey.OrganizationId)
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
		Issuer: infra.MockFirebaseIssuer,
		Email:  "user@email.com",
	}
	token := "token"
	now := time.Now()

	user := models.User{
		UserId:         "user_id",
		Email:          "user@email.com",
		Role:           models.ADMIN,
		OrganizationId: utils.TextToUUID("organization_id"),
	}
	orgIdString := user.OrganizationId

	t.Run("nominal", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("Verify", mock.Anything, firebaseToken).
			Return(user, firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("ActiveGrantsForPrincipal", mock.Anything, "user", string(user.UserId)).
			Return([]models.Grant{{Role: models.ADMIN, OrganizationId: user.OrganizationId}}, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, orgIdString).
			Return(models.Organization{}, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", infra.MockFirebaseIssuer, mock.Anything, models.Credentials{
			OrganizationId: utils.TextToUUID("organization_id"),
			Role:           models.ADMIN,
			Roles:          []models.Role{models.ADMIN},
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
			Return(user, firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("ActiveGrantsForPrincipal", mock.Anything, "user", string(user.UserId)).
			Return([]models.Grant{{Role: models.ADMIN, OrganizationId: user.OrganizationId}}, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, orgIdString).
			Return(models.Organization{}, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", infra.MockFirebaseIssuer, mock.Anything, models.Credentials{
			OrganizationId: utils.TextToUUID("organization_id"),
			Role:           models.ADMIN,
			Roles:          []models.Role{models.ADMIN},
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

	t.Run("EncodeMarbleToken error", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("Verify", mock.Anything, firebaseToken).
			Return(user, firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("ActiveGrantsForPrincipal", mock.Anything, "user", string(user.UserId)).
			Return([]models.Grant{{Role: models.ADMIN, OrganizationId: user.OrganizationId}}, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, orgIdString).
			Return(models.Organization{}, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", infra.MockFirebaseIssuer, mock.Anything, models.Credentials{
			OrganizationId: utils.TextToUUID("organization_id"),
			Role:           models.ADMIN,
			Roles:          []models.Role{models.ADMIN},
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

	t.Run("legacy role and grants union", func(t *testing.T) {
		tenantId := utils.TextToUUID("tenant_id")

		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("Verify", mock.Anything, firebaseToken).
			Return(user, firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("ActiveGrantsForPrincipal", mock.Anything, "user", string(user.UserId)).
			Return([]models.Grant{
				{Role: models.TENANT_ADMIN, TenantId: tenantId},
				{Role: models.ADMIN, OrganizationId: user.OrganizationId},
			}, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, orgIdString).
			Return(models.Organization{TenantId: tenantId}, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", infra.MockFirebaseIssuer, mock.Anything, models.Credentials{
			OrganizationId: orgIdString,
			TenantId:       tenantId,
			Role:           models.ADMIN,
			Roles:          []models.Role{models.ADMIN, models.TENANT_ADMIN},
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
		mockRepository.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})

	t.Run("marble admin platform token", func(t *testing.T) {
		admin := models.User{
			UserId: "admin_id",
			Email:  "admin@email.com",
			Role:   models.MARBLE_ADMIN,
		}

		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("Verify", mock.Anything, firebaseToken).
			Return(admin, firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("ActiveGrantsForPrincipal", mock.Anything, "user", string(admin.UserId)).
			Return([]models.Grant{}, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", infra.MockFirebaseIssuer, mock.Anything, models.Credentials{
			Role:  models.MARBLE_ADMIN,
			Roles: []models.Role{models.MARBLE_ADMIN},
			ActorIdentity: models.Identity{
				UserId: admin.UserId,
				Email:  admin.Email,
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
		mockRepository.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})
}
