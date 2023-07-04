package repositories

import (
	"crypto/rsa"
	"fmt"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"time"

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

var VALIDATION_ALGO = jwt.SigningMethodRS256

func (repo *MarbleJwtRepository) EncodeMarbleToken(expirationTime time.Time, creds models.Credentials) (string, error) {

	claims := &Claims{
		Credentials: dto.AdaptCredentialDto(creds),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Issuer:    "marble",
		},
	}

	token := jwt.NewWithClaims(VALIDATION_ALGO, claims)

	tokenString, err := token.SignedString(&repo.jwtSigningPrivateKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (repo *MarbleJwtRepository) ValidateMarbleToken(marbleToken string) (models.Credentials, error) {
	token, err := jwt.ParseWithClaims(marbleToken, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		method, ok := token.Method.(*jwt.SigningMethodRSA)
		if !ok || method != VALIDATION_ALGO {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return &repo.jwtSigningPrivateKey.PublicKey, nil
	})

	if err != nil {
		return models.Credentials{}, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return dto.AdaptCredential(claims.Credentials), nil
	} else {
		return models.Credentials{}, fmt.Errorf("invalid Marble Jwt Token")
	}
}
