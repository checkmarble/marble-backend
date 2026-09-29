package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type DBCaseManualEntity struct {
	Id        uuid.UUID `db:"id"`
	OrgId     uuid.UUID `db:"org_id"`
	CaseId    uuid.UUID `db:"case_id"`
	TableName string    `db:"table_name"`
	ObjectId  string    `db:"object_id"`
}

func AdaptCaseManualEntity(db DBCaseManualEntity) models.CaseManualEntity {
	return models.CaseManualEntity{
		Id: db.Id.String(), OrganizationId: db.OrgId, CaseId: db.CaseId.String(),
		CaseEntityRef: models.CaseEntityRef{TableName: db.TableName, ObjectId: db.ObjectId},
	}
}
