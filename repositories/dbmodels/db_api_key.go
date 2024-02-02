package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgtype"
)

type DBApiKey struct {
	Id             string             `db:"id"`
	OrganizationId string             `db:"org_id"`
	Hash           string             `db:"key"`
	Description    string             `db:"description"`
	DeletedAt      pgtype.Timestamptz `db:"deleted_at"`
	Role           int                `db:"role"`
}

const TABLE_APIKEYS = "apikeys"

var ApiKeyFields = utils.ColumnList[DBApiKey]()

func AdaptApikey(db DBApiKey) (models.ApiKey, error) {
	return models.ApiKey{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		Hash:           db.Hash,
		Description:    db.Description,
		Role:           models.Role(db.Role),
	}, nil
}
