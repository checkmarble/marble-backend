package dbmodels

import "github.com/google/uuid"

const TABLE_GRANTS = "grants"

type DbTenantGrant struct {
	Id                 uuid.UUID `db:"id"`
	PrincipalType      string    `db:"principal_type"`
	PrincipalId        string    `db:"principal_id"`
	PrincipalAuthority string    `db:"principal_authority"`
	TenantId           uuid.UUID `db:"tenant_id"`
	Role               string    `db:"role"`
}
