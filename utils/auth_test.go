package utils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) ValidateTokenOrKey(ctx context.Context, marbleToken, apiKey string) (models.Credentials, error) {
	args := m.Called(ctx, marbleToken, apiKey)
	return args.Get(0).(models.Credentials), args.Error(1)
}

func TestAuthedBy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		methods        []AuthType
		setupHeaders   func(*http.Request)
		setupValidator func(*MockValidator)
		expectedStatus int
	}{
		{
			name:    "success with PublicApiKey",
			methods: []AuthType{PublicApiKey},
			setupHeaders: func(r *http.Request) {
				r.Header.Set("X-API-Key", "test-api-key")
			},
			setupValidator: func(v *MockValidator) {
				v.On("ValidateTokenOrKey", mock.Anything, "", "test-api-key").
					Return(models.Credentials{
						ActorIdentity: models.Identity{ApiKeyName: "test"},
						Role:          models.ADMIN,
					}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "success with BearerToken",
			methods: []AuthType{ApiKeyAsBearerToken},
			setupHeaders: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer test-token")
			},
			setupValidator: func(v *MockValidator) {
				v.On("ValidateTokenOrKey", mock.Anything, "", "test-token").
					Return(models.Credentials{
						ActorIdentity: models.Identity{Email: "test@example.com"},
						Role:          models.VIEWER,
					}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "success with FederatedBearerToken",
			methods: []AuthType{FederatedBearerToken},
			setupHeaders: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer test-jwt")
			},
			setupValidator: func(v *MockValidator) {
				v.On("ValidateTokenOrKey", mock.Anything, "test-jwt", "").
					Return(models.Credentials{
						ActorIdentity: models.Identity{Email: "test@example.com"},
						Role:          models.VIEWER,
					}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "invalid bearer token format",
			methods: []AuthType{ApiKeyAsBearerToken},
			setupHeaders: func(r *http.Request) {
				r.Header.Set("Authorization", "InvalidFormat")
			},
			setupValidator: func(v *MockValidator) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:    "unauthorized when validation fails",
			methods: []AuthType{PublicApiKey},
			setupHeaders: func(r *http.Request) {
				r.Header.Set("X-API-Key", "invalid-key")
			},
			setupValidator: func(v *MockValidator) {
				v.On("ValidateTokenOrKey", mock.Anything, "", "invalid-key").
					Return(models.Credentials{}, models.NotFoundError)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:    "empty authorization header",
			methods: []AuthType{ApiKeyAsBearerToken},
			setupHeaders: func(r *http.Request) {
				// Don't set any headers
			},
			setupValidator: func(v *MockValidator) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:    "multiple auth methods - none provided",
			methods: []AuthType{ApiKeyAsBearerToken, PublicApiKey},
			setupHeaders: func(r *http.Request) {
				// Don't set any headers
			},
			setupValidator: func(v *MockValidator) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:    "multiple auth methods - valid API key",
			methods: []AuthType{ApiKeyAsBearerToken, PublicApiKey},
			setupHeaders: func(r *http.Request) {
				r.Header.Set("X-API-Key", "test-api-key")
			},
			setupValidator: func(v *MockValidator) {
				v.On("ValidateTokenOrKey", mock.Anything, "", "test-api-key").
					Return(models.Credentials{
						ActorIdentity: models.Identity{ApiKeyName: "test"},
						Role:          models.ADMIN,
					}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "success with ScreeningIndexerToken",
			methods: []AuthType{ScreeningIndexerToken},
			setupHeaders: func(r *http.Request) {
				r.Header.Set("Authorization", "Token test-indexer-token")
			},
			setupValidator: func(v *MockValidator) {},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "unauthorized with wrong ScreeningIndexerToken",
			methods: []AuthType{ScreeningIndexerToken},
			setupHeaders: func(r *http.Request) {
				r.Header.Set("Authorization", "Token wrong-token")
			},
			setupValidator: func(v *MockValidator) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:    "bad request with malformed ScreeningIndexerToken",
			methods: []AuthType{ScreeningIndexerToken},
			setupHeaders: func(r *http.Request) {
				r.Header.Set("Authorization", "Tokenmalformed")
			},
			setupValidator: func(v *MockValidator) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:    "fallback to API key when ScreeningIndexerToken is missing but allowed",
			methods: []AuthType{ScreeningIndexerToken, PublicApiKey},
			setupHeaders: func(r *http.Request) {
				r.Header.Set("X-API-Key", "test-api-key")
			},
			setupValidator: func(v *MockValidator) {
				v.On("ValidateTokenOrKey", mock.Anything, "", "test-api-key").
					Return(models.Credentials{
						ActorIdentity: models.Identity{ApiKeyName: "test"},
						Role:          models.ADMIN,
					}, nil)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := new(MockValidator)
			tt.setupValidator(validator)
			auth := NewAuthentication(validator, "test-indexer-token")

			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)

			// Setup the route with auth middleware and handler
			engine.GET("/test", auth.AuthedBy(tt.methods...), func(c *gin.Context) {
				if tt.expectedStatus == http.StatusOK {
					// ScreeningIndexerToken doesn't set credentials in context
					// We only check for credentials if the Token header was not used
					authHeader := c.Request.Header.Get("Authorization")
					isIndexerTokenSuccess := false
					for _, m := range tt.methods {
						if m == ScreeningIndexerToken && strings.HasPrefix(authHeader, "Token ") {
							isIndexerTokenSuccess = true
							break
						}
					}

					if !isIndexerTokenSuccess {
						creds, exists := c.Request.Context().Value(
							ContextKeyCredentials).(models.Credentials)
						assert.True(t, exists, "credentials should be set in context")
						assert.NotEmpty(t, creds, "credentials should not be empty")
					}
				}
				c.Status(http.StatusOK)
			})

			// Create and setup the request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			tt.setupHeaders(req)

			// Process the request through the engine
			engine.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			validator.AssertExpectations(t)
		})
	}
}

func TestParseAuthorizationTokenHeader(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		expectedToken string
		expectedErr   bool
	}{
		{
			name:          "valid token",
			authorization: "Token my-secret-token",
			expectedToken: "my-secret-token",
			expectedErr:   false,
		},
		{
			name:          "empty header",
			authorization: "",
			expectedToken: "",
			expectedErr:   false,
		},
		{
			name:          "malformed token - no space",
			authorization: "Tokenmytoken",
			expectedToken: "",
			expectedErr:   true,
		},
		{
			name:          "malformed token - wrong prefix",
			authorization: "Bearer mytoken",
			expectedToken: "",
			expectedErr:   true,
		},
		{
			name:          "token with spaces - split behavior",
			authorization: "Token my Token has spaces",
			expectedToken: "my Token has spaces",
			expectedErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			if tt.authorization != "" {
				header.Set("Authorization", tt.authorization)
			}
			token, err := ParseAuthorizationTokenHeader(header)
			if tt.expectedErr {
				assert.Error(t, err)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedToken, token)
			}
		})
	}
}
