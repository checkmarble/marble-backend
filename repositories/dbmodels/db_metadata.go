package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DBMetadata struct {
	ID        uuid.UUID  `db:"id"`
	CreatedAt time.Time  `db:"created_at"`
	OrgID     *uuid.UUID `db:"org_id"`
	Key       string     `db:"key"`
	Value     string     `db:"value"`
}

const TABLE_METADATA = "metadata"

var MetadataFields = utils.ColumnList[DBMetadata]()

func AdaptMetadata(db DBMetadata) (models.Metadata, error) {
	key, err := models.MetadataKeyFromString(db.Key)
	if err != nil {
		return models.Metadata{}, err
	}

	return models.Metadata{
		ID:        db.ID,
		CreatedAt: db.CreatedAt,
		OrgID:     db.OrgID,
		Key:       key,
		Value:     db.Value,
	}, nil
}
