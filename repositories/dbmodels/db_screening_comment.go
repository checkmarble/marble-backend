package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SCREENING_MATCH_COMMENTS = "screening_match_comments"

var SelectScreeningMatchCommentsColumn = utils.ColumnList[DBScreeningMatchComment]()

type DBScreeningMatchComment struct {
	Id               string    `db:"id"`
	ScreeningMatchId string    `db:"screening_match_id"`
	CommentedBy      string    `db:"commented_by"`
	Comment          string    `db:"comment"`
	CreatedAt        time.Time `db:"created_at"`
}

func AdaptScreeningMatchComment(dto DBScreeningMatchComment) (models.ScreeningMatchComment, error) {
	return models.ScreeningMatchComment{
		Id:          dto.Id,
		MatchId:     dto.ScreeningMatchId,
		CommenterId: models.UserId(dto.CommentedBy),
		Comment:     dto.Comment,
		CreatedAt:   dto.CreatedAt,
	}, nil
}
