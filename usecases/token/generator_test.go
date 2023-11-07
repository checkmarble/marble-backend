package token

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/clock"
)

func TestGenerator_GenerateToken_APIKey(t *testing.T) {
	apiKeyString := "api_key"
	key := models.ApiKey{
		ApiKeyId:       "api_key_id",
		OrganizationId: "organization_id",
		Key:            apiKeyString,
		Role:           models.ADMIN,
	}

	organization := models.Organization{
		Id:           "organization_id",
		Name:         "organization",
		DatabaseName: "organization_database",
	}

	token := "token"
	now := time.Now()

	t.Run("nominal", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByKey", mock.Anything, apiKeyString).
			Return(key, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, "organization_id").
			Return(organization, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", mock.Anything, models.Credentials{
			OrganizationId: "organization_id",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				ApiKeyName: "ApiKey Of organization",
			},
		}).
			Return(token, nil)

		generator := Generator{
			repository:    mockRepository,
			encoder:       mockEncoder,
			clock:         clock.NewMock(now),
			tokenLifetime: 60 * time.Second,
		}

		receivedToken, expirationTime, err := generator.GenerateToken(context.Background(), apiKeyString, "")
		assert.NoError(t, err)
		assert.Equal(t, token, receivedToken)
		assert.Equal(t, now.Add(60*time.Second), expirationTime)

		mockRepository.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})

	t.Run("GetApiKeyByKey error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByKey", mock.Anything, apiKeyString).
			Return(models.ApiKey{}, assert.AnError)

		generator := Generator{
			repository: mockRepository,
		}

		_, _, err := generator.GenerateToken(context.Background(), apiKeyString, "")
		assert.Error(t, err)

		mockRepository.AssertExpectations(t)
	})

	t.Run("GetOrganizationByID error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByKey", mock.Anything, apiKeyString).
			Return(key, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, "organization_id").
			Return(models.Organization{}, assert.AnError)

		generator := Generator{
			repository: mockRepository,
		}

		_, _, err := generator.GenerateToken(context.Background(), apiKeyString, "")
		assert.Error(t, err)

		mockRepository.AssertExpectations(t)
	})

	t.Run("EncodeMarbleToken error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByKey", mock.Anything, apiKeyString).
			Return(key, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, "organization_id").
			Return(organization, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", mock.Anything, models.Credentials{
			OrganizationId: "organization_id",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				ApiKeyName: "ApiKey Of organization",
			},
		}).
			Return(token, nil)

		generator := Generator{
			repository:    mockRepository,
			encoder:       mockEncoder,
			clock:         clock.NewMock(now),
			tokenLifetime: 60 * time.Second,
		}

		receivedToken, expirationTime, err := generator.GenerateToken(context.Background(), apiKeyString, "")
		assert.NoError(t, err)
		assert.Equal(t, token, receivedToken)
		assert.Equal(t, now.Add(60*time.Second), expirationTime)

		mockRepository.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})
}

func TestGenerator_GenerateToken_FirebaseToken(t *testing.T) {
	firebaseToken := "firebaseToken"
	firebaseIdentity := models.FirebaseIdentity{
		Email:       "user@email.com",
		FirebaseUid: "firebase_uid",
	}
	token := "token"
	now := time.Now()

	user := models.User{
		UserId:         "user_id",
		Email:          "user@email.com",
		FirebaseUid:    "firebase_uid",
		Role:           models.ADMIN,
		OrganizationId: "organization_id",
	}

	t.Run("nominal", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("VerifyFirebaseToken", mock.Anything, firebaseToken).
			Return(firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("UserByFirebaseUid", mock.Anything, firebaseIdentity.FirebaseUid).
			Return(user, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, "organization_id").
			Return(models.Organization{}, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", mock.Anything, models.Credentials{
			OrganizationId: "organization_id",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				UserId: user.UserId,
				Email:  user.Email,
			},
		}).
			Return(token, nil)

		generator := Generator{
			repository:    mockRepository,
			verifier:      mockVerifier,
			encoder:       mockEncoder,
			clock:         clock.NewMock(now),
			tokenLifetime: 60 * time.Second,
		}

		receivedToken, expirationTime, err := generator.GenerateToken(context.Background(), "", firebaseToken)
		assert.NoError(t, err)
		assert.Equal(t, token, receivedToken)
		assert.Equal(t, now.Add(60*time.Second), expirationTime)
		mockRepository.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})

	t.Run("nominal first connection", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("VerifyFirebaseToken", mock.Anything, firebaseToken).
			Return(firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("UserByFirebaseUid", mock.Anything, firebaseIdentity.FirebaseUid).
			Return(models.User{}, models.NotFoundError)
		mockRepository.On("UserByEmail", mock.Anything, firebaseIdentity.Email).
			Return(user, nil)
		mockRepository.On("UpdateUserFirebaseUID", mock.Anything, user.UserId, firebaseIdentity.FirebaseUid).
			Return(nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, "organization_id").
			Return(models.Organization{}, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", mock.Anything, models.Credentials{
			OrganizationId: "organization_id",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				UserId: user.UserId,
				Email:  user.Email,
			},
		}).
			Return(token, nil)

		generator := Generator{
			repository:    mockRepository,
			verifier:      mockVerifier,
			encoder:       mockEncoder,
			clock:         clock.NewMock(now),
			tokenLifetime: 60 * time.Second,
		}

		receivedToken, expirationTime, err := generator.GenerateToken(context.Background(), "", firebaseToken)
		assert.NoError(t, err)
		assert.Equal(t, token, receivedToken)
		assert.Equal(t, now.Add(60*time.Second), expirationTime)
		mockRepository.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})

	t.Run("VerifyFirebaseToken error", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("VerifyFirebaseToken", mock.Anything, firebaseToken).
			Return(models.FirebaseIdentity{}, assert.AnError)

		generator := Generator{
			verifier: mockVerifier,
		}

		_, _, err := generator.GenerateToken(context.Background(), "", firebaseToken)
		assert.Error(t, err)
		mockVerifier.AssertExpectations(t)
	})

	t.Run("UserByFirebaseUid error", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("VerifyFirebaseToken", mock.Anything, firebaseToken).
			Return(firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("UserByFirebaseUid", mock.Anything, firebaseIdentity.FirebaseUid).
			Return(models.User{}, assert.AnError)

		generator := Generator{
			repository: mockRepository,
			verifier:   mockVerifier,
		}

		_, _, err := generator.GenerateToken(context.Background(), "", firebaseToken)
		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
	})

	t.Run("UserByEmail error", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("VerifyFirebaseToken", mock.Anything, firebaseToken).
			Return(firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("UserByFirebaseUid", mock.Anything, firebaseIdentity.FirebaseUid).
			Return(models.User{}, models.NotFoundError)
		mockRepository.On("UserByEmail", mock.Anything, firebaseIdentity.Email).
			Return(models.User{}, assert.AnError)

		generator := Generator{
			repository: mockRepository,
			verifier:   mockVerifier,
		}

		_, _, err := generator.GenerateToken(context.Background(), "", firebaseToken)
		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
	})

	t.Run("UpdateUserFirebaseUID error", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("VerifyFirebaseToken", mock.Anything, firebaseToken).
			Return(firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("UserByFirebaseUid", mock.Anything, firebaseIdentity.FirebaseUid).
			Return(models.User{}, models.NotFoundError)
		mockRepository.On("UserByEmail", mock.Anything, firebaseIdentity.Email).
			Return(user, nil)
		mockRepository.On("UpdateUserFirebaseUID", mock.Anything, user.UserId, firebaseIdentity.FirebaseUid).
			Return(assert.AnError)

		generator := Generator{
			repository: mockRepository,
			verifier:   mockVerifier,
		}

		_, _, err := generator.GenerateToken(context.Background(), "", firebaseToken)
		assert.Error(t, err)
		mockVerifier.AssertExpectations(t)
		mockRepository.AssertExpectations(t)
	})

	t.Run("EncodeMarbleToken error", func(t *testing.T) {
		mockVerifier := new(mocks.FirebaseTokenVerifier)
		mockVerifier.On("VerifyFirebaseToken", mock.Anything, firebaseToken).
			Return(firebaseIdentity, nil)

		mockRepository := new(mocks.Database)
		mockRepository.On("UserByFirebaseUid", mock.Anything, firebaseIdentity.FirebaseUid).
			Return(models.User{}, models.NotFoundError)
		mockRepository.On("UserByEmail", mock.Anything, firebaseIdentity.Email).
			Return(user, nil)
		mockRepository.On("UpdateUserFirebaseUID", mock.Anything, user.UserId, firebaseIdentity.FirebaseUid).
			Return(nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", mock.Anything, models.Credentials{
			OrganizationId: "organization_id",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				UserId: user.UserId,
				Email:  user.Email,
			},
		}).
			Return("", assert.AnError)

		generator := Generator{
			repository:    mockRepository,
			verifier:      mockVerifier,
			encoder:       mockEncoder,
			clock:         clock.NewMock(now),
			tokenLifetime: 60 * time.Second,
		}

		_, _, err := generator.GenerateToken(context.Background(), "", firebaseToken)
		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
		mockEncoder.AssertExpectations(t)
	})
}
