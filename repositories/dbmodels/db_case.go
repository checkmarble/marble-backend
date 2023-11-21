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
	DecisionsCount pgtype.Int4      `db:"decisions_count"`
}

type DBCaseWithContributors struct {
	DBCase
	Contributors []DBCaseContributor `db:"contributors"`
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
		DecisionsCount: int(db.DecisionsCount.Int32),
	}, nil
}

func AdaptCasewithContributors(db DBCaseWithContributors) (models.Case, error) {
	caseModel, err := AdaptCase(db.DBCase)
	if err != nil {
		return models.Case{}, err
	}

	caseModel.Contributors = make([]models.CaseContributor, len(db.Contributors))
	for i, contributor := range db.Contributors {
		caseModel.Contributors[i], err = AdaptCaseContributor(contributor)
		if err != nil {
			return models.Case{}, err
		}
	}

	return caseModel, nil
}
