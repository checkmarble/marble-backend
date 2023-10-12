package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
)

type FirebaseTokenVerifier struct {
	mock.Mock
}

func (m *FirebaseTokenVerifier) VerifyFirebaseToken(ctx context.Context, firebaseToken string) (models.FirebaseIdentity, error) {
	args := m.Called(ctx, firebaseToken)
	return args.Get(0).(models.FirebaseIdentity), args.Error(1)
}
