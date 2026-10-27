package repositories

import (
	"crypto/rsa"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"

	"github.com/golang-jwt/jwt/v4"
)

type MarbleJwtRepository struct {
	issuers              map[string]struct{}
	jwtSigningPrivateKey rsa.PrivateKey
}

// We add jwt.RegisteredClaims as an embedded type, to provide fields like expiry time
type Claims struct {
	jwt.RegisteredClaims

	IdentityIssuer string          `json:"identity_issuer"`
	Credentials    dto.Credentials `json:"credentials"`
}

var ValidationAlgo = jwt.SigningMethodRS256

func (repo *MarbleJwtRepository) EncodeMarbleToken(issuer string, expirationTime time.Time, creds models.Credentials) (string, error) {
	credDto, err := dto.AdaptCredentialDto(creds)
	if err != nil {
		return "", err
	}
	claims := &Claims{
		IdentityIssuer: issuer,
		Credentials:    credDto,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Issuer:    "marble",
		},
	}

	// TODO: no component, to this day, uses the permissions contained in the
	// token, so let's empty it to regain spaces in the cookie. This cannot be
	// completely removed since components still expect to find an array.
	claims.Credentials.Permissions = make([]string, 0)

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
	if _, ok := repo.issuers[claims.RegisteredClaims.Issuer]; !ok {
		return models.Credentials{}, errors.Wrapf(models.UnAuthorizedError,
			"invalid Marble token issuer %q", claims.RegisteredClaims.Issuer)
	}
	if ok && token.Valid {
		return dto.AdaptCredential(claims.Credentials), nil
	}
	return models.Credentials{}, errors.Errorf("invalid Marble Jwt Token")
}

func NewJWTRepository(issuer string, key *rsa.PrivateKey) *MarbleJwtRepository {
	return NewJWTRepositoryForIssuers([]string{issuer}, key)
}

func NewJWTRepositoryForIssuers(issuers []string, key *rsa.PrivateKey) *MarbleJwtRepository {
	issuerSet := make(map[string]struct{}, len(issuers))
	for _, issuer := range issuers {
		issuerSet[issuer] = struct{}{}
	}

	return &MarbleJwtRepository{
		issuers:              issuerSet,
		jwtSigningPrivateKey: *key,
	}
}
