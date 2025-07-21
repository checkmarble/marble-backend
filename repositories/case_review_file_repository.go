package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
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

func (r *MarbleDbRepository) ListCaseReviewFiles(
	ctx context.Context,
	exec Executor,
	caseId uuid.UUID,
) ([]models.AiCaseReviewFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.AiCaseReviewFileFields...).
		From(dbmodels.TABLE_AI_CASE_REVIEW_FILES).
		Where(squirrel.Eq{
			"case_id": caseId,
			"status":  models.AiCaseReviewFileStatusCompleted.String(),
		}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		func(dbModel dbmodels.AiCaseReviewFile) (models.AiCaseReviewFile, error) {
			return dbmodels.AdaptAiCaseReviewFile(dbModel), nil
		},
	)
}
