package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DBEntityAnnotation struct {
	Id             string          `db:"id"`
	OrgId          uuid.UUID       `db:"org_id"`
	ObjectType     string          `db:"object_type"`
	ObjectId       string          `db:"object_id"`
	CaseId         *string         `db:"case_id"`
	AnnotationType string          `db:"annotation_type"`
	Payload        json.RawMessage `db:"payload"`
	AnnotatedBy    *string         `db:"annotated_by"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
	DeletedAt      *time.Time      `db:"deleted_at"`
}

const TABLE_ENTITY_ANNOTATIONS = "entity_annotations"

var EntityAnnotationColumns = utils.ColumnList[DBEntityAnnotation]()

func AdaptEntityAnnotation(db DBEntityAnnotation) (models.EntityAnnotation, error) {
	var userId *models.UserId
	if db.AnnotatedBy != nil {
		userId = utils.Ptr(models.UserId(*db.AnnotatedBy))
	}

	return models.EntityAnnotation{
		Id:             db.Id,
		OrgId:          db.OrgId,
		ObjectType:     db.ObjectType,
		ObjectId:       db.ObjectId,
		CaseId:         db.CaseId,
		AnnotationType: models.EntityAnnotationFrom(db.AnnotationType),
		Payload:        db.Payload,
		AnnotatedBy:    userId,
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
		DeletedAt:      db.DeletedAt,
	}, nil
}
