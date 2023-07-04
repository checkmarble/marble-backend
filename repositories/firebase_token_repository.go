package repositories

import (
	"context"
	"fmt"

	"marble/marble-backend/models"

	"firebase.google.com/go/v4/auth"
)

type FireBaseTokenRepository struct {
	firebaseClient auth.Client
}

func (repo *FireBaseTokenRepository) VerifyFirebaseToken(ctx context.Context, firebaseToken string) (models.FirebaseIdentity, error) {
	token, err := repo.firebaseClient.VerifyIDToken(ctx, firebaseToken)
	if err != nil {
		token, err = repo.firebaseClient.VerifySessionCookie(ctx, firebaseToken)
	}
	if err != nil {
		return models.FirebaseIdentity{}, err
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
