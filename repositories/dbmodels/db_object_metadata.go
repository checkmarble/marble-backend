package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DBObjectMetadata struct {
	Id           uuid.UUID       `db:"id"`
	OrgId        uuid.UUID       `db:"org_id"`
	ObjectType   string          `db:"object_type"`
	ObjectId     string          `db:"object_id"`
	MetadataType string          `db:"metadata_type"`
	Metadata     json.RawMessage `db:"metadata"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

const TABLE_OBJECT_METADATA = "object_metadata"

var SelectObjectMetadataColumn = utils.ColumnList[DBObjectMetadata]()

func AdaptObjectMetadata(db DBObjectMetadata) (models.ObjectMetadata, error) {
	metadataType := models.MetadataTypeFrom(db.MetadataType)
	metadata, err := models.ParseMetadataContent(metadataType, db.Metadata)
	if err != nil {
		return models.ObjectMetadata{}, err
	}

	return models.ObjectMetadata{
		Id:           db.Id,
		OrgId:        db.OrgId,
		ObjectType:   db.ObjectType,
		ObjectId:     db.ObjectId,
		MetadataType: metadataType,
		Metadata:     metadata,
		CreatedAt:    db.CreatedAt,
		UpdatedAt:    db.UpdatedAt,
	}, nil
}

