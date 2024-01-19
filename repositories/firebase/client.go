package firebase

import (
	"context"
	"fmt"

	"firebase.google.com/go/v4/auth"

	"github.com/checkmarble/marble-backend/models"
)

type tokenCookieVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error)
	VerifySessionCookie(ctx context.Context, sessionCookie string) (*auth.Token, error)
}

type Client struct {
	verifier tokenCookieVerifier
}

func (c *Client) verifyTokenOrCookie(ctx context.Context, firebaseToken string) (*auth.Token, error) {
	token, err := c.verifier.VerifyIDToken(ctx, firebaseToken)
	if err != nil {
		token, err = c.verifier.VerifySessionCookie(ctx, firebaseToken)
	}
	if err != nil {
		return nil, err
	}
	if token.Firebase.SignInProvider == "password" && token.Claims["email_verified"] == false {
		return nil, fmt.Errorf("email not verified")
	}

	return token, nil
}

func (c *Client) VerifyFirebaseToken(ctx context.Context, firebaseToken string) (models.FirebaseIdentity, error) {
	token, err := c.verifyTokenOrCookie(ctx, firebaseToken)
	if err != nil {
		return models.FirebaseIdentity{}, fmt.Errorf("verifyTokenOrCookie error: %w", err)
	}

	identities := token.Firebase.Identities["email"]
	if identities == nil {
		return models.FirebaseIdentity{}, fmt.Errorf("unexpected firebase token content: Field email is missing")
	}

	emails, ok := identities.([]interface{})
	if !ok || len(emails) == 0 {
		return models.FirebaseIdentity{}, fmt.Errorf("unexpected firebase token content: identities is not an array")
	}

	email, ok := emails[0].(string)
	if !ok {
		return models.FirebaseIdentity{}, fmt.Errorf("unexpected firebase token content")
	}

	return models.FirebaseIdentity{
		Email:       email,
		FirebaseUid: token.Subject,
	}, nil
}

func New(verifier tokenCookieVerifier) *Client {
	return &Client{
		verifier: verifier,
	}
}
