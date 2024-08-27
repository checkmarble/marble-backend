package infra

import (
	"context"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/cockroachdb/errors"
)

func InitializeFirebase(ctx context.Context) *auth.Client {
	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		panic(errors.Wrap(err, "error initializing app"))
	}

	client, err := app.Auth(ctx)
	if err != nil {
		panic(errors.Wrap(err, "error getting Auth client"))
	}

	return client
}

type mockedTokenVerifier struct{}

func (m mockedTokenVerifier) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	return &auth.Token{
		Firebase: auth.FirebaseInfo{
			Identities:     map[string]interface{}{"email": []string{"test@test.com"}},
			SignInProvider: "password",
		},
		Claims: map[string]interface{}{
			"email_verified": true,
		},
	}, nil
}

func NewMockedFirebaseTokenVerifier() mockedTokenVerifier {
	return mockedTokenVerifier{}
}
