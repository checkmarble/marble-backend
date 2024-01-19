package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

type FireBaseTokenRepository interface {
	VerifyFirebaseToken(ctx context.Context, firebaseToken string) (models.FirebaseIdentity, error)
}
