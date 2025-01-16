package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SANCTION_CHECK_MATCH_COMMENTS = "sanction_check_match_comments"

var SelectSanctionCheckMatchCommentsColumn = utils.ColumnList[DBSanctionCheckMatchComment]()

type DBSanctionCheckMatchComment struct {
	Id                   string    `db:"id"`
	SanctionCheckMatchId string    `db:"sanction_check_match_id"`
	CommentedBy          string    `db:"commented_by"`
	Comment              string    `db:"comment"`
	CreatedAt            time.Time `db:"created_at"`
}
