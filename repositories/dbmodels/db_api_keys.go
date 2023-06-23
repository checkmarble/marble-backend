package dbmodels

import (
	"marble/marble-backend/models"

	"github.com/jackc/pgx/v5/pgtype"
)

type DBApiKey struct {
	ID        string      `db:"id"`
	OrgID     string      `db:"org_id"`
	Key       string      `db:"key"`
	DeletedAt pgtype.Time `db:"deleted_at"`
	Role      int         `db:"role"`
}

var ApiKeyFields = []string{"id", "org_id", "key", "deleted_at", "role"}

const TABLE_APIKEYS = "apikeys"

func AdaptApikey(db DBApiKey) models.ApiKey {
	return models.ApiKey{
		ApiKeyId:       models.ApiKeyId(db.ID),
		OrganizationId: db.OrgID,
		Key:            db.Key,
		Role:           models.Role(db.Role),
	}
}
