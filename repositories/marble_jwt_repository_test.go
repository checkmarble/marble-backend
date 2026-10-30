package repositories

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarbleJwtRepository_ValidateMarbleToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	repository := NewJWTRepository("marble", key)

	for _, tt := range []struct {
		name           string
		identityIssuer string
		marbleIssuer   string
		wantErr        bool
	}{
		{name: "accepts Firebase identity issuer", identityIssuer: "firebase-issuer", marbleIssuer: "marble"},
		{name: "accepts tenant OIDC identity issuer", identityIssuer: "http://localhost:5556/dex", marbleIssuer: "marble"},
		{name: "rejects a non-Marble token issuer", identityIssuer: "oidc-issuer", marbleIssuer: "other", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			authTenantID := uuid.New()
			token, err := repository.EncodeMarbleToken(tt.identityIssuer, time.Now().Add(time.Hour), models.Credentials{AuthTenantId: authTenantID})
			require.NoError(t, err)
			if tt.marbleIssuer == "marble" {
				var claims Claims
				_, _, err = new(jwt.Parser).ParseUnverified(token, &claims)
				require.NoError(t, err)
				assert.Equal(t, tt.identityIssuer, claims.IdentityIssuer)
				assert.Equal(t, "marble", claims.RegisteredClaims.Issuer)

				rawClaims := jwt.MapClaims{}
				_, _, err = new(jwt.Parser).ParseUnverified(token, rawClaims)
				require.NoError(t, err)
				assert.Contains(t, rawClaims, "identity_issuer")
				assert.NotContains(t, rawClaims, "issuer")
			}
			if tt.marbleIssuer != "marble" {
				claims := &Claims{RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)), Issuer: tt.marbleIssuer}}
				token, err = jwt.NewWithClaims(ValidationAlgo, claims).SignedString(key)
				require.NoError(t, err)
			}

			credentials, err := repository.ValidateMarbleToken(token)
			if tt.wantErr {
				assert.ErrorIs(t, err, models.UnAuthorizedError)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, authTenantID, credentials.AuthTenantId)
		})
	}
}
