package infra

import (
	"context"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"gocloud.dev/secrets"
	"gocloud.dev/secrets/localsecrets"
)

const OidcSecretsKeyEnv = "OIDC_SECRETS_KEY"

func InitializeOidcSecretsKeeper(ctx context.Context) (*secrets.Keeper, error) {
	key := utils.GetEnv(OidcSecretsKeyEnv, "")
	if key == "" {
		return nil, errors.New(OidcSecretsKeyEnv + " is required when OpenID Connect is enabled")
	}

	secretKey, err := localsecrets.Base64Key(key)
	if err != nil {
		return nil, errors.Wrap(err, "decoding OIDC secrets key")
	}
	return localsecrets.NewKeeper(secretKey), nil
}
