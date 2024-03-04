package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgtype"
)

type DBApiKey struct {
	Id             string             `db:"id"`
	CreatedAt      time.Time          `db:"created_at"`
	OrganizationId string             `db:"org_id"`
	Key            string             `db:"key"` // TODO(hash-key): alter column name to "hash"
	Hash           []byte             `db:"key_hash"`
	Description    string             `db:"description"`
	DeletedAt      pgtype.Timestamptz `db:"deleted_at"`
	Role           int                `db:"role"`
}

const TABLE_APIKEYS = "apikeys"

var ApiKeyFields = utils.ColumnList[DBApiKey]()

func AdaptApikey(db DBApiKey) (models.ApiKey, error) {
	return models.ApiKey{
		Id:             db.Id,
		CreatedAt:      db.CreatedAt,
		Description:    db.Description,
		Key:            db.Key,
		Hash:           db.Hash,
		OrganizationId: db.OrganizationId,
		Role:           models.Role(db.Role),
	}, nil
}
