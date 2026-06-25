package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SCREENING_MATCH_COMMENTS = "screening_match_comments"

var SelectScreeningMatchCommentsColumn = utils.ColumnList[DBScreeningMatchComment]()

// A comment belongs to either a (transaction monitoring) screening match or a continuous screening
// match: exactly one of ScreeningMatchId / ContinuousScreeningMatchId is set (enforced by a CHECK
// constraint). Both are nullable here to reflect that.
type DBScreeningMatchComment struct {
	Id                         string    `db:"id"`
	ScreeningMatchId           *string   `db:"screening_match_id"`
	ContinuousScreeningMatchId *string   `db:"continuous_screening_match_id"`
	CommentedBy                string    `db:"commented_by"`
	Comment                    string    `db:"comment"`
	CreatedAt                  time.Time `db:"created_at"`
}

func AdaptScreeningMatchComment(dto DBScreeningMatchComment) (models.ScreeningMatchComment, error) {
	matchId := ""
	switch {
	case dto.ScreeningMatchId != nil:
		matchId = *dto.ScreeningMatchId
	case dto.ContinuousScreeningMatchId != nil:
		matchId = *dto.ContinuousScreeningMatchId
	}

	return models.ScreeningMatchComment{
		Id:          dto.Id,
		MatchId:     matchId,
		CommenterId: models.UserId(dto.CommentedBy),
		Comment:     dto.Comment,
		CreatedAt:   dto.CreatedAt,
	}, nil
}
