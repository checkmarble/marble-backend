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
	DeletedAt      pgtype.Timestamptz `db:"deleted_at"`
	Description    string             `db:"description"`
	Hash           []byte             `db:"key_hash"`
	Prefix         string             `db:"prefix"`
	PartnerId      pgtype.Text        `db:"partner_id"`
	OrganizationId string             `db:"org_id"`
	Role           int                `db:"role"`
}

const TABLE_APIKEYS = "api_keys"

var ApiKeyFields = utils.ColumnList[DBApiKey]()

func AdaptApikey(db DBApiKey) (models.ApiKey, error) {
	out := models.ApiKey{
		Id:             db.Id,
		CreatedAt:      db.CreatedAt,
		Description:    db.Description,
		Hash:           db.Hash,
		OrganizationId: db.OrganizationId,
		Prefix:         db.Prefix,
		Role:           models.Role(db.Role),
	}
	if db.PartnerId.Valid {
		out.PartnerId = &db.PartnerId.String
	}

	return out, nil
}
