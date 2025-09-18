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
	issuer               string
	jwtSigningPrivateKey rsa.PrivateKey
}

// We add jwt.RegisteredClaims as an embedded type, to provide fields like expiry time
type Claims struct {
	jwt.RegisteredClaims

	Issuer      string          `json:"issuer"`
	Credentials dto.Credentials `json:"credentials"`
}

var ValidationAlgo = jwt.SigningMethodRS256

func (repo *MarbleJwtRepository) EncodeMarbleToken(issuer string, expirationTime time.Time, creds models.Credentials) (string, error) {
	credDto, err := dto.AdaptCredentialDto(creds)
	if err != nil {
		return "", err
	}
	claims := &Claims{
		Issuer:      issuer,
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

	claims, ok := token.Claims.(*Claims)
	if claims.Issuer != repo.issuer {
		return models.Credentials{}, errors.Newf("invalid token issuer '%s'", claims.Issuer)
	}

	if ok && token.Valid {
		return dto.AdaptCredential(claims.Credentials), nil
	}
	return models.Credentials{}, fmt.Errorf("invalid Marble Jwt Token")
}

func NewJWTRepository(issuer string, key *rsa.PrivateKey) *MarbleJwtRepository {
	return &MarbleJwtRepository{
		issuer:               issuer,
		jwtSigningPrivateKey: *key,
	}
}
