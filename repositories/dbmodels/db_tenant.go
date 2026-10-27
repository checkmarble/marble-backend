package dbmodels

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const TABLE_TENANTS = "tenants"

type DBTenant struct {
	Id   uuid.UUID `db:"id"`
	Name string    `db:"name"`
}

func AdaptTenant(db DBTenant) (models.Tenant, error) {
	return models.Tenant{Id: db.Id, Name: db.Name}, nil
}

// DBTenantWithOIDCConfiguration is a tenant left-joined with its OIDC
// configuration: the OIDC columns are NULL when the tenant has none. The client
// secret is never selected.
type DBTenantWithOIDCConfiguration struct {
	DBTenant
	OIDCIssuer      *string  `db:"oidc_issuer"`
	OIDCClientId    *string  `db:"oidc_client_id"`
	OIDCScopes      []string `db:"oidc_scopes"`
	OIDCExtraParams []byte   `db:"oidc_extra_params"`
	OIDCEmailClaim  *string  `db:"oidc_email_claim"`
	OIDCSsoEnabled  *bool    `db:"oidc_sso_enabled"`
}

func AdaptTenantWithOIDCConfiguration(db DBTenantWithOIDCConfiguration) (models.Tenant, error) {
	tenant, err := AdaptTenant(db.DBTenant)
	if err != nil || db.OIDCIssuer == nil {
		return tenant, err
	}
	config := models.TenantOIDCConfiguration{
		TenantId:   db.Id,
		Issuer:     *db.OIDCIssuer,
		ClientId:   utils.Or(db.OIDCClientId, ""),
		Scopes:     db.OIDCScopes,
		EmailClaim: utils.Or(db.OIDCEmailClaim, ""),
		SsoEnabled: utils.Or(db.OIDCSsoEnabled, false),
	}
	if err := json.Unmarshal(db.OIDCExtraParams, &config.ExtraParams); err != nil {
		return tenant, errors.Wrap(err, "decoding tenant OIDC extra parameters")
	}
	tenant.OIDCConfiguration = &config
	return tenant, nil
}
