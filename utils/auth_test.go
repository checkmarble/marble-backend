package utils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) Validate(ctx context.Context, marbleToken, apiKey string) (models.Credentials, error) {
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
				v.On("Validate", mock.Anything, "", "test-api-key").
					Return(models.Credentials{
						ActorIdentity: models.Identity{ApiKeyName: "test"},
						Role:          models.ADMIN,
					}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "success with BearerToken",
			methods: []AuthType{BearerToken},
			setupHeaders: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer test-token")
			},
			setupValidator: func(v *MockValidator) {
				v.On("Validate", mock.Anything, "", "test-token").
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
				v.On("Validate", mock.Anything, "test-jwt", "").
					Return(models.Credentials{
						ActorIdentity: models.Identity{Email: "test@example.com"},
						Role:          models.VIEWER,
					}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "invalid bearer token format",
			methods: []AuthType{BearerToken},
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
				v.On("Validate", mock.Anything, "", "invalid-key").
					Return(models.Credentials{}, models.NotFoundError)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:    "empty authorization header",
			methods: []AuthType{BearerToken},
			setupHeaders: func(r *http.Request) {
				// Don't set any headers
			},
			setupValidator: func(v *MockValidator) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:    "multiple auth methods - none provided",
			methods: []AuthType{BearerToken, PublicApiKey},
			setupHeaders: func(r *http.Request) {
				// Don't set any headers
			},
			setupValidator: func(v *MockValidator) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:    "multiple auth methods - valid API key",
			methods: []AuthType{BearerToken, PublicApiKey},
			setupHeaders: func(r *http.Request) {
				r.Header.Set("X-API-Key", "test-api-key")
			},
			setupValidator: func(v *MockValidator) {
				v.On("Validate", mock.Anything, "", "test-api-key").
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
			auth := NewAuthentication(validator)

			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)

			// Setup the route with auth middleware and handler
			engine.GET("/test", auth.AuthedBy(tt.methods...), func(c *gin.Context) {
				if tt.expectedStatus == http.StatusOK {
					creds, exists := c.Request.Context().Value(
						ContextKeyCredentials).(models.Credentials)
					assert.True(t, exists, "credentials should be set in context")
					assert.NotEmpty(t, creds, "credentials should not be empty")
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
