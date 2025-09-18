package idp

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

type TokenRepository interface {
	Issuer() string
	VerifyToken(ctx context.Context, firebaseToken string) (models.IdentityClaims, error)
}
