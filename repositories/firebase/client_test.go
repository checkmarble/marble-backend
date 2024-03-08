package firebase

import (
	"context"
	"testing"

	"firebase.google.com/go/v4/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
)

type mockTokenCookieVerifier struct {
	mock.Mock
}

func (m *mockTokenCookieVerifier) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	args := m.Called(ctx, idToken)
	return args.Get(0).(*auth.Token), args.Error(1)
}

func TestClient_VerifyFirebaseToken(t *testing.T) {
	token := auth.Token{
		Subject: "token_subject",
		Firebase: auth.FirebaseInfo{
			Identities: map[string]interface{}{
				"email": []interface{}{"user@email.com"},
			},
		},
	}

	t.Run("nominal", func(t *testing.T) {
		mockVerifier := new(mockTokenCookieVerifier)
		mockVerifier.On("VerifyIDToken", mock.Anything, "token").
			Return(&token, nil)

		c := Client{
			verifier: mockVerifier,
		}

		identity, err := c.VerifyFirebaseToken(context.Background(), "token")
		assert.NoError(t, err)
		assert.Equal(t, models.FirebaseIdentity{
			Email: "user@email.com",
		}, identity)
		mockVerifier.AssertExpectations(t)
	})

	t.Run("VerifyIDToken error", func(t *testing.T) {
		mockVerifier := new(mockTokenCookieVerifier)
		mockVerifier.On("VerifyIDToken", mock.Anything, "token").
			Return(&auth.Token{}, assert.AnError)

		c := Client{
			verifier: mockVerifier,
		}

		_, err := c.VerifyFirebaseToken(context.Background(), "token")
		assert.Error(t, err)
		mockVerifier.AssertExpectations(t)
	})

	t.Run("no email identities in token", func(t *testing.T) {
		mockVerifier := new(mockTokenCookieVerifier)
		mockVerifier.On("VerifyIDToken", mock.Anything, "token").
			Return(&auth.Token{}, nil)

		c := Client{
			verifier: mockVerifier,
		}

		_, err := c.VerifyFirebaseToken(context.Background(), "token")
		assert.Error(t, err)
		mockVerifier.AssertExpectations(t)
	})

	t.Run("identities is not an array", func(t *testing.T) {
		mockVerifier := new(mockTokenCookieVerifier)
		mockVerifier.On("VerifyIDToken", mock.Anything, "token").
			Return(&auth.Token{
				Subject: "token_subject",
				Firebase: auth.FirebaseInfo{
					Identities: map[string]interface{}{
						"email": "user@email.com",
					},
				},
			}, nil)

		c := Client{
			verifier: mockVerifier,
		}

		_, err := c.VerifyFirebaseToken(context.Background(), "token")
		assert.Error(t, err)
		mockVerifier.AssertExpectations(t)
	})

	t.Run("email is not a string", func(t *testing.T) {
		mockVerifier := new(mockTokenCookieVerifier)
		mockVerifier.On("VerifyIDToken", mock.Anything, "token").
			Return(&auth.Token{
				Subject: "token_subject",
				Firebase: auth.FirebaseInfo{
					Identities: map[string]interface{}{
						"email": []interface{}{42},
					},
				},
			}, nil)

		c := Client{
			verifier: mockVerifier,
		}

		_, err := c.VerifyFirebaseToken(context.Background(), "token")
		assert.Error(t, err)
		mockVerifier.AssertExpectations(t)
	})
}
