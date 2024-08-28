package infra

import (
	"context"

	"firebase.google.com/go/v4/auth"
)

type mockedTokenVerifier struct{}

func (m mockedTokenVerifier) VerifyIDToken(_ context.Context, email string) (*auth.Token, error) {
	return &auth.Token{
		Firebase: auth.FirebaseInfo{
			Identities:     map[string]any{"email": []any{email}},
			SignInProvider: "password",
		},
		Claims: map[string]any{
			"email_verified": true,
		},
	}, nil
}

func NewMockedFirebaseTokenVerifier() mockedTokenVerifier { //nolint:revive
	return mockedTokenVerifier{}
}
