package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBCaseContributor struct {
	Id        string    `db:"id"`
	CaseId    string    `db:"case_id"`
	UserId    string    `db:"user_id"`
	CreatedAt time.Time `db:"created_at"`
}

const TABLE_CASE_CONTRIBUTORS = "case_contributors"

var SelectCaseContributorColumn = utils.ColumnList[DBCaseContributor]()

func AdaptCaseContributor(caseContributor DBCaseContributor) (models.CaseContributor, error) {
	return models.CaseContributor{
		Id:        caseContributor.Id,
		CaseId:    caseContributor.CaseId,
		UserId:    caseContributor.UserId,
		CreatedAt: caseContributor.CreatedAt,
	}, nil
}
