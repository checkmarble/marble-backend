package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/auth"
)

type FirebaseTokenVerifier struct {
	mock.Mock
}

func (m *FirebaseTokenVerifier) VerifyFirebaseToken(ctx context.Context, firebaseToken string) (models.FirebaseIdentity, error) {
	args := m.Called(ctx, firebaseToken)
	return args.Get(0).(models.FirebaseIdentity), args.Error(1)
}

func (m *FirebaseTokenVerifier) Verify(ctx context.Context, creds auth.Credentials) (models.IdentityClaims, error) {
	args := m.Called(ctx, creds)
	return args.Get(0).(models.FirebaseIdentity), args.Error(1)
}

type FirebaseAdminClient struct {
	mock.Mock
}

func (m *FirebaseAdminClient) CreateUser(ctx context.Context, email, name string) error {
	args := m.Called(ctx, email, name)

	return args.Error(0)
}
