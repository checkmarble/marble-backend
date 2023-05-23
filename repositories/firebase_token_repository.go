package repositories

import (
	"context"
	"fmt"

	. "marble/marble-backend/models"

	"firebase.google.com/go/v4/auth"
)

type FireBaseTokenRepository struct {
	firebaseClient auth.Client
}

func (repo *FireBaseTokenRepository) VerifyFirebaseIDToken(ctx context.Context, firebaseIdToken string) (FirebaseIdentity, error) {
	token, err := repo.firebaseClient.VerifyIDToken(ctx, firebaseIdToken)
	if err != nil {
		return FirebaseIdentity{}, err
	}
	identities := token.Firebase.Identities["email"]
	if identities == nil {
		return FirebaseIdentity{}, fmt.Errorf("Unexpected firebase IdToken content: Field email is missing.")
	}

	emails, ok := identities.([]interface{})
	if !ok || len(emails) == 0 {
		return FirebaseIdentity{}, fmt.Errorf("Unexpected firebase IdToken content: identities is not an array.")
	}

	email, ok := emails[0].(string)
	if !ok {
		return FirebaseIdentity{}, fmt.Errorf("Unexpected firebase IdToken content.")
	}

	return FirebaseIdentity{
		Email:       email,
		FirebaseUid: token.Subject,
	}, nil
}
