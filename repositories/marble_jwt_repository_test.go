package repositories

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarbleJwtRepository_ValidateMarbleToken_AllowedIssuers(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	repository := NewJWTRepositoryForIssuers([]string{"firebase-issuer", "oidc-issuer"}, key)
	otherRepository := NewJWTRepository("other-issuer", key)

	for _, tt := range []struct {
		name       string
		issuer     string
		repository *MarbleJwtRepository
		wantErr    bool
	}{
		{name: "accepts Firebase issuer", issuer: "firebase-issuer", repository: repository},
		{name: "accepts OIDC issuer", issuer: "oidc-issuer", repository: repository},
		{name: "rejects issuer outside configured set", issuer: "other-issuer", repository: otherRepository, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			token, err := tt.repository.EncodeMarbleToken(tt.issuer, time.Now().Add(time.Hour), models.Credentials{})
			require.NoError(t, err)

			_, err = repository.ValidateMarbleToken(token)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
