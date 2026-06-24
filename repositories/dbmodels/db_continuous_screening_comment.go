package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_CONTINUOUS_SCREENING_MATCH_COMMENTS = "continuous_screening_match_comments"

var SelectContinuousScreeningMatchCommentsColumn = utils.ColumnList[DBContinuousScreeningMatchComment]()

type DBContinuousScreeningMatchComment struct {
	Id                         uuid.UUID `db:"id"`
	ContinuousScreeningMatchId uuid.UUID `db:"continuous_screening_match_id"`
	CommentedBy                uuid.UUID `db:"commented_by"`
	Comment                    string    `db:"comment"`
	CreatedAt                  time.Time `db:"created_at"`
}

func AdaptContinuousScreeningMatchComment(dto DBContinuousScreeningMatchComment) (models.ScreeningMatchComment, error) {
	return models.ScreeningMatchComment{
		Id:          dto.Id.String(),
		MatchId:     dto.ContinuousScreeningMatchId.String(),
		CommenterId: models.UserId(dto.CommentedBy.String()),
		Comment:     dto.Comment,
		CreatedAt:   dto.CreatedAt,
	}, nil
}
