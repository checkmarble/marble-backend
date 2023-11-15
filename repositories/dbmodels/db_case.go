package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBCase struct {
	Id             string    `db:"id"`
	OrganizationId string    `db:"org_id"`
	CreatedAt      time.Time `db:"created_at"`
	Name           string    `db:"name"`
	Description    string    `db:"description"`
	Status         string    `db:"status"`
}

const TABLE_CASES = "cases"

var SelectCaseColumn = utils.ColumnList[DBCase]()

func AdaptCase(db DBCase) (models.Case, error) {
	return models.Case{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		CreatedAt:      db.CreatedAt,
		Name:           db.Name,
		Description:    db.Description,
		Status:         models.CaseStatusFrom(db.Status),
	}, nil
}

func AdaptCaseExtended(db DBCase, decisions []models.Decision) (models.Case) {
	return models.Case{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		CreatedAt:      db.CreatedAt,
		Name:           db.Name,
		Description:    db.Description,
		Status:         models.CaseStatusFrom(db.Status),
		Decisions:      decisions,
	}
}
