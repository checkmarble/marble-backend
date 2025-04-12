package repositories

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"

	"github.com/golang-jwt/jwt/v4"
)

type MarbleJwtRepository struct {
	jwtSigningPrivateKey rsa.PrivateKey
}

// We add jwt.RegisteredClaims as an embedded type, to provide fields like expiry time
type Claims struct {
	Credentials dto.Credentials `json:"credentials"`
	jwt.RegisteredClaims
}

var ValidationAlgo = jwt.SigningMethodRS256

func (repo *MarbleJwtRepository) EncodeMarbleToken(expirationTime time.Time, creds models.Credentials) (string, error) {
	credDto, err := dto.AdaptCredentialDto(creds)
	if err != nil {
		return "", err
	}
	claims := &Claims{
		Credentials: credDto,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Issuer:    "marble",
		},
	}

	token := jwt.NewWithClaims(ValidationAlgo, claims)
	return token.SignedString(&repo.jwtSigningPrivateKey)
}

func (repo *MarbleJwtRepository) ValidateMarbleToken(marbleToken string) (models.Credentials, error) {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		method, ok := token.Method.(*jwt.SigningMethodRSA)
		if !ok || method != ValidationAlgo {
			return nil, errors.Wrapf(models.UnAuthorizedError,
				"unexpected signing method: %v", token.Header["alg"])
		}
		return &repo.jwtSigningPrivateKey.PublicKey, nil
	}

	token, err := jwt.ParseWithClaims(marbleToken, &Claims{}, keyFunc)
	if err != nil {
		return models.Credentials{}, errors.Join(
			models.UnAuthorizedError,
			errors.Wrapf(err, "Error parsing jwt token claims"),
		)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return dto.AdaptCredential(claims.Credentials), nil
	}
	return models.Credentials{}, fmt.Errorf("invalid Marble Jwt Token")
}

func NewJWTRepository(key *rsa.PrivateKey) *MarbleJwtRepository {
	return &MarbleJwtRepository{
		jwtSigningPrivateKey: *key,
	}
}
