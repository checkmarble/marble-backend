package auth_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"log/slog"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/clock"
	"github.com/checkmarble/marble-backend/repositories/idp"
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGenerator_VerifyToken_APIKey(t *testing.T) {
	key := "api_key"
	// hash of "api_key"
	keyHash, err := hex.DecodeString("2e9bc6c94a4cbdfe2a31d2df79103a5eb3702eaf5d7018d47a774e9540a8ec29")
	assert.NoError(t, err)

	apiKey := models.ApiKey{
		Id:             "api_key_id",
		OrganizationId: utils.TextToUUID("organization_id"),
		Prefix:         "abc",
		Role:           models.ADMIN,
		DisplayString:  "Api key abc*** of organization",
	}
	orgIdString := apiKey.OrganizationId

	organization := models.Organization{
		Id:   utils.TextToUUID("organization_id"),
		Name: "organization",
	}

	ctx := t.Context()

	idpTokenVerifier := mocks.NewStaticIdpTokenVerifier(infra.MockFirebaseIssuer, models.FirebaseIdentity{
		Issuer: infra.MockFirebaseIssuer, Email: "user@email.com",
	})

	t.Run("nominal", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByHash", ctx, keyHash).
			Return(apiKey, nil)
		mockRepository.On("GetOrganizationByID", ctx, orgIdString).
			Return(organization, nil)

		verifier := auth.NewVerifier(map[string]idp.TokenRepository{infra.MockFirebaseIssuer: idpTokenVerifier}, mockRepository, "", nil)
		intoCreds, _, err := verifier.Verify(ctx, auth.Credentials{
			Type: auth.CredentialsApiKey, Value: key,
		})

		assert.NoError(t, err)
		assert.Equal(t, apiKey.Id, intoCreds.IntoCredentials().ActorIdentity.ApiKeyId)
		assert.Equal(t, apiKey.OrganizationId, intoCreds.IntoCredentials().OrganizationId)
	})

	t.Run("GetApiKeyByHash error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByHash", ctx, keyHash).
			Return(models.ApiKey{}, assert.AnError)

		verifier := auth.NewVerifier(map[string]idp.TokenRepository{infra.MockFirebaseIssuer: idpTokenVerifier}, mockRepository, "", nil)
		_, _, err := verifier.Verify(ctx, auth.Credentials{Type: auth.CredentialsApiKey, Value: key})

		assert.Error(t, err)

		mockRepository.AssertExpectations(t)
	})

	t.Run("GetOrganizationByID error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetApiKeyByHash", ctx, keyHash).
			Return(apiKey, nil)
		mockRepository.On("GetOrganizationByID", ctx, orgIdString).
			Return(models.Organization{}, assert.AnError)

		verifier := auth.NewVerifier(map[string]idp.TokenRepository{infra.MockFirebaseIssuer: idpTokenVerifier}, mockRepository, "", nil)
		_, _, err := verifier.Verify(ctx, auth.Credentials{Type: auth.CredentialsApiKey, Value: key})

		assert.Error(t, err)

		mockRepository.AssertExpectations(t)
	})
}

func TestParseTokenProviders(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    auth.TokenProviders
		wantErr bool
	}{
		{name: "defaults to Firebase", want: auth.TokenProviders{auth.TokenProviderFirebase}},
		{name: "supports multiple providers", value: "firebase,oidc", want: auth.TokenProviders{auth.TokenProviderFirebase, auth.TokenProviderOidc}},
		{name: "trims whitespace", value: " firebase, oidc ", want: auth.TokenProviders{auth.TokenProviderFirebase, auth.TokenProviderOidc}},
		{name: "rejects unknown provider", value: "firebase,unknown", wantErr: true},
		{name: "rejects duplicate provider", value: "firebase,firebase", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := auth.ParseTokenProviders(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMarbleVerifier_Verify_RejectsUnconfiguredIssuer(t *testing.T) {
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, auth.BaseClaims{Issuer: "https://unknown.example.com"}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	verifier := auth.NewVerifier(map[string]idp.TokenRepository{}, new(mocks.Database), "", nil)
	_, _, err = verifier.Verify(t.Context(), auth.Credentials{Type: auth.CredentialsBearer, Value: token})

	assert.ErrorContains(t, err, "no identity provider is configured")
}

func TestMarbleVerifier_Verify_RejectsTenantAuthenticationWithoutRegistry(t *testing.T) {
	issuer := "https://id.example.test"
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, auth.BaseClaims{Issuer: issuer}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	verifier := auth.NewVerifier(map[string]idp.TokenRepository{issuer: mocks.NewStaticIdpTokenVerifier(issuer, models.OidcIdentity{})}, new(mocks.Database), issuer, nil)
	_, _, err = verifier.Verify(t.Context(), auth.Credentials{Type: auth.CredentialsBearer, Value: token, TenantId: uuid.New()})

	require.ErrorContains(t, err, "tenant OIDC registry is not configured")
}

func TestGenerator_VerifyToken_FirebaseToken(t *testing.T) {
	firebaseJWT, err := jwt.NewWithClaims(jwt.SigningMethodNone, auth.BaseClaims{Issuer: infra.MockFirebaseIssuer}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	assert.NoError(t, err)
	firebaseToken := auth.Credentials{Type: auth.CredentialsBearer, Value: firebaseJWT}
	firebaseIdentity := models.FirebaseIdentity{
		Issuer:  infra.MockFirebaseIssuer,
		Subject: "firebase_subject",
		Email:   "user@email.com",
	}
	token := "token"
	now := time.Now()

	user := models.User{
		UserId:         "user_id",
		Email:          "user@email.com",
		Role:           models.ADMIN,
		OrganizationId: utils.TextToUUID("organization_id"),
	}
	userOrgIdString := user.OrganizationId

	idpTokenVerifier := mocks.NewStaticIdpTokenVerifier(infra.MockFirebaseIssuer, firebaseIdentity)

	t.Run("nominal", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("ActiveGrantsForPrincipal", mock.Anything, "user", string(user.UserId)).
			Return([]models.Grant{{Role: models.ADMIN, OrganizationId: user.OrganizationId}}, nil)
		mockRepository.On("GetOrganizationByID", mock.Anything, userOrgIdString).
			Return(models.Organization{}, nil)
		mockRepository.On("ResolveUserIdentity", mock.Anything, firebaseIdentity.Issuer, firebaseIdentity.Subject, firebaseIdentity.Email).
			Return(user, nil)
		mockRepository.On("UpdateUserProfileFromClaims", mock.Anything, user, models.IdentityUpdatableClaims{}).
			Return(user, nil)

		mockEncoder := new(mocks.JWTEncoderValidator)
		mockEncoder.On("EncodeMarbleToken", infra.MockFirebaseIssuer, mock.Anything, models.Credentials{
			OrganizationId: utils.TextToUUID("organization_id"),
			Roles:          []models.Role{models.ADMIN},
			ActorIdentity: models.Identity{
				UserId: user.UserId,
				Email:  user.Email,
			},
		}).
			Return(token, nil)

		verifier := auth.NewVerifier(map[string]idp.TokenRepository{infra.MockFirebaseIssuer: idpTokenVerifier}, mockRepository, "", nil)
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

	t.Run("ResolveUserIdentity error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("ResolveUserIdentity", mock.Anything, firebaseIdentity.Issuer, firebaseIdentity.Subject, firebaseIdentity.Email).
			Return(models.User{}, assert.AnError)

		verifier := auth.NewVerifier(map[string]idp.TokenRepository{infra.MockFirebaseIssuer: idpTokenVerifier}, mockRepository, "", nil)
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

func TestMarbleVerifier_LogsEmailMismatch(t *testing.T) {
	const (
		storedEmail = "old@example.test"
		idpEmail    = "new@example.test"
	)
	identity := models.FirebaseIdentity{
		Issuer:  infra.MockFirebaseIssuer,
		Subject: "subject",
		Email:   idpEmail,
	}
	user := models.User{UserId: "user-id", Email: storedEmail}
	mockRepository := new(mocks.Database)
	mockRepository.On("ResolveUserIdentity", mock.Anything, identity.Issuer, identity.Subject, identity.Email).
		Return(user, nil)
	mockRepository.On("UpdateUserProfileFromClaims", mock.Anything, user, models.IdentityUpdatableClaims{}).
		Return(user, nil)

	var logOutput bytes.Buffer
	ctx := utils.StoreLoggerInContext(t.Context(), slog.New(slog.NewJSONHandler(&logOutput, nil)))
	verifier := auth.NewVerifier(
		map[string]idp.TokenRepository{identity.Issuer: mocks.NewStaticIdpTokenVerifier(identity.Issuer, identity)},
		mockRepository,
		"",
		nil,
	)
	credential, err := jwt.NewWithClaims(jwt.SigningMethodNone, auth.BaseClaims{Issuer: identity.Issuer}).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, _, err = verifier.Verify(ctx, auth.Credentials{Type: auth.CredentialsBearer, Value: credential})
	require.NoError(t, err)
	assert.Contains(t, logOutput.String(), "identity email mismatch")
	assert.Contains(t, logOutput.String(), identity.Issuer)
	assert.Contains(t, logOutput.String(), string(user.UserId))
	assert.NotContains(t, logOutput.String(), storedEmail)
	assert.NotContains(t, logOutput.String(), idpEmail)
	mockRepository.AssertExpectations(t)
}
