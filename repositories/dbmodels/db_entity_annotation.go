package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBEntityAnnotation struct {
	Id             string          `db:"id"`
	OrgId          string          `db:"org_id"`
	ObjectType     string          `db:"object_type"`
	ObjectId       string          `db:"object_id"`
	AnnotationType string          `db:"annotation_type"`
	Payload        json.RawMessage `db:"payload"`
	AttachedBy     *string         `db:"attached_by"`
	CreatedAt      time.Time       `db:"created_at"`
	DeletedAt      *time.Time      `db:"deleted_at"`
}

const TABLE_ENTITY_ATTACHMENTS = "entity_annotations"

var EntityAnnotationColumns = utils.ColumnList[DBEntityAnnotation]()

func AdaptEntityAnnotation(db DBEntityAnnotation) (models.EntityAnnotation, error) {
	var userId *models.UserId
	if db.AttachedBy != nil {
		userId = utils.Ptr(models.UserId(*db.AttachedBy))
	}

	return models.EntityAnnotation{
		Id:             db.Id,
		OrgId:          db.OrgId,
		ObjectType:     db.ObjectType,
		ObjectId:       db.ObjectId,
		AnnotationType: models.EntityAnnotationFrom(db.AnnotationType),
		Payload:        db.Payload,
		AttachedBy:     userId,
		CreatedAt:      db.CreatedAt,
		DeletedAt:      db.DeletedAt,
	}, nil
}
