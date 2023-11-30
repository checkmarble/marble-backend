package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

type DBCaseTag struct {
	Id        string           `db:"id"`
	CaseId    string           `db:"case_id"`
	TagId     string           `db:"tag_id"`
	CreatedAt time.Time        `db:"created_at"`
	DeletedAt pgtype.Timestamp `db:"deleted_at"`
}

const TABLE_CASE_TAGS = "case_tags"

var SelectCaseTagColumn = utils.ColumnList[DBCaseTag]()

func AdaptCaseTag(db DBCaseTag) (models.CaseTag, error) {
	return models.CaseTag{
		Id:        db.Id,
		CaseId:    db.CaseId,
		TagId:     db.TagId,
		CreatedAt: db.CreatedAt,
	}, nil
}
