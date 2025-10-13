package idp

import (
	"context"
	"fmt"

	"firebase.google.com/go/v4/auth"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type firebaseTokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error)
}

type FirebaseClient struct {
	projectId string
	verifier  firebaseTokenVerifier
}

func (c *FirebaseClient) verifyTokenOrCookie(ctx context.Context, firebaseToken string) (*auth.Token, error) {
	token, err := c.verifier.VerifyIDToken(ctx, firebaseToken)
	if err != nil {
		return nil, err
	}
	if token.Firebase.SignInProvider == "password" && token.Claims["email_verified"] == false {
		return nil, fmt.Errorf("email not verified")
	}

	return token, nil
}

func (c *FirebaseClient) VerifyToken(ctx context.Context, firebaseToken string) (models.IdentityClaims, error) {
	token, err := c.verifyTokenOrCookie(ctx, firebaseToken)
	if err != nil {
		return models.FirebaseIdentity{}, fmt.Errorf("firebaseVerifyToken error: %w", err)
	}

	if token.Issuer != c.Issuer() {
		return models.FirebaseIdentity{}, fmt.Errorf("invalid issuer %s != %s for firebase", token.Issuer, c.Issuer())
	}

	email, ok := pure_utils.AnySliceAtIndex[string](token.Firebase.Identities["email"], 0)
	if !ok {
		return models.FirebaseIdentity{}, fmt.Errorf(
			"unexpected firebase token content: Field email is missing")
	}

	picture := ""

	if p, ok := token.Claims["picture"].(string); ok {
		picture = p
	}

	return models.FirebaseIdentity{
		Issuer:  token.Issuer,
		Email:   email,
		Picture: picture,
	}, nil
}

func (c *FirebaseClient) Issuer() string {
	return "https://securetoken.google.com/" + c.projectId
}

func NewFirebaseClient(projectId string, verifier firebaseTokenVerifier) *FirebaseClient {
	return &FirebaseClient{
		projectId: projectId,
		verifier:  verifier,
	}
}
