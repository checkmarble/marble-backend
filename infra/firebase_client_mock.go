package infra

import (
	"context"

	"firebase.google.com/go/v4/auth"
	"github.com/golang-jwt/jwt/v4"
)

const (
	MockFirebaseIssuer = "https://securetoken.google.com/project"
)

type mockedTokenVerifier struct{}

func (m mockedTokenVerifier) VerifyIDToken(_ context.Context, idToken string) (*auth.Token, error) {
	var claims jwt.MapClaims

	if _, _, err := new(jwt.Parser).ParseUnverified(idToken, &claims); err != nil {
		return nil, err
	}
	subject, _ := claims["sub"].(string)

	return &auth.Token{
		Issuer:  MockFirebaseIssuer,
		Subject: subject,
		Firebase: auth.FirebaseInfo{
			Identities:     map[string]any{"email": []any{claims["email"]}},
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
