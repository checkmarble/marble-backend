package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (r *MarbleDbRepository) CreateCaseReviewFile(
	ctx context.Context,
	exec Executor,
	caseReview models.AiCaseReviewFile,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_AI_CASE_REVIEW_FILES).
			Columns(
				"id",
				"case_id",
				"status",
				"bucket_name",
				"file_reference",
				"dto_version",
			).
			Values(
				caseReview.ID,
				caseReview.CaseID,
				caseReview.Status,
				caseReview.BucketName,
				caseReview.FileReference,
				"v1",
			),
	)
	return err
}
