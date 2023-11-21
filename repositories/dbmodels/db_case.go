package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

type DBCase struct {
	Id             pgtype.Text      `db:"id"`
	OrganizationId pgtype.Text      `db:"org_id"`
	CreatedAt      pgtype.Timestamp `db:"created_at"`
	Name           pgtype.Text      `db:"name"`
	Status         pgtype.Text      `db:"status"`
}

const TABLE_CASES = "cases"

var SelectCaseColumn = utils.ColumnList[DBCase]()

func AdaptCase(db DBCase) (models.Case, error) {
	return models.Case{
		Id:             db.Id.String,
		OrganizationId: db.OrganizationId.String,
		CreatedAt:      db.CreatedAt.Time,
		Name:           db.Name.String,
		Status:         models.CaseStatus(db.Status.String),
	}, nil
}
