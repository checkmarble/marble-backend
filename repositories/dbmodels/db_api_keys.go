package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"

	"github.com/jackc/pgx/v5/pgtype"
)

type DBApiKey struct {
	Id             string      `db:"id"`
	OrganizationId string      `db:"org_id"`
	Key            string      `db:"key"`
	DeletedAt      pgtype.Time `db:"deleted_at"`
	Role           int         `db:"role"`
}

var ApiKeyFields = []string{"id", "org_id", "key", "deleted_at", "role"}

const TABLE_APIKEYS = "apikeys"

func AdaptApikey(db DBApiKey) models.ApiKey {
	return models.ApiKey{
		ApiKeyId:       models.ApiKeyId(db.Id),
		OrganizationId: db.OrganizationId,
		Key:            db.Key,
		Role:           models.Role(db.Role),
	}
}
