package mocks

import (
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
)

type JWTEncoderValidator struct {
	mock.Mock
}

func (m *JWTEncoderValidator) EncodeMarbleToken(expirationTime time.Time, creds models.Credentials) (string, error) {
	args := m.Called(expirationTime, creds)
	return args.String(0), args.Error(1)
}

func (m *JWTEncoderValidator) ValidateMarbleToken(marbleToken string) (models.Credentials, error) {
	args := m.Called(marbleToken)
	return args.Get(0).(models.Credentials), args.Error(1)
}
