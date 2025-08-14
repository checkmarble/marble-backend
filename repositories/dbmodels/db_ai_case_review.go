package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type AiCaseReview struct {
	Id     uuid.UUID `db:"id"`
	CaseId uuid.UUID `db:"case_id"`

	Status            string `db:"status"`
	FileReference     string `db:"file_reference"`
	FileTempReference string `db:"file_temp_reference"`
	BucketName        string `db:"bucket_name"`
	DtoVersion        string `db:"dto_version"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	Reaction *string `db:"reaction"`
}

const TABLE_AI_CASE_REVIEWS = "ai_case_reviews"

var AiCaseReviewFields = utils.ColumnList[AiCaseReview]()

func AdaptAiCaseReview(dbModel AiCaseReview) (models.AiCaseReview, error) {
	var reaction *models.AiCaseReviewReaction
	if dbModel.Reaction != nil {
		reaction = utils.Ptr(models.AiCaseReviewReactionFromString(*dbModel.Reaction))
	}

	return models.AiCaseReview{
		Id:                dbModel.Id,
		CaseId:            dbModel.CaseId,
		Status:            models.AiCaseReviewStatusFromString(dbModel.Status),
		BucketName:        dbModel.BucketName,
		FileReference:     dbModel.FileReference,
		FileTempReference: dbModel.FileTempReference,
		DtoVersion:        dbModel.DtoVersion,
		CreatedAt:         dbModel.CreatedAt,
		UpdatedAt:         dbModel.UpdatedAt,
		AiCaseReviewFeedback: models.AiCaseReviewFeedback{
			Reaction: reaction,
		},
	}, nil
}
