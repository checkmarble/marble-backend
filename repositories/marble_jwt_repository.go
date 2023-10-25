package repositories

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
)

type MarbleJwtRepository struct {
	jwtSigningPrivateKey rsa.PrivateKey
}

type Claims struct {
	jwt.RegisteredClaims
	Credentials dto.Credentials `json:"credentials"`
}

var ValidationAlgo = jwt.SigningMethodRS256

func (repo *MarbleJwtRepository) EncodeMarbleToken(expirationTime time.Time, creds models.Credentials) (string, error) {
	claims := &Claims{
		Credentials: dto.AdaptCredentialDto(creds),
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
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return &repo.jwtSigningPrivateKey.PublicKey, nil
	}

	token, err := jwt.ParseWithClaims(marbleToken, &Claims{}, keyFunc)
	if err != nil {
		return models.Credentials{}, err
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
