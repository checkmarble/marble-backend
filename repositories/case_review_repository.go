package repositories

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) CreateCaseReviewFile(
	ctx context.Context,
	exec Executor,
	caseReview models.AiCaseReview,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_AI_CASE_REVIEWS).
			Columns(
				"id",
				"case_id",
				"status",
				"bucket_name",
				"file_reference",
				"file_temp_reference",
				"dto_version",
				"reaction",
			).
			Values(
				caseReview.Id,
				caseReview.CaseId,
				caseReview.Status,
				caseReview.BucketName,
				caseReview.FileReference,
				caseReview.FileTempReference,
				caseReview.DtoVersion,
				caseReview.Reaction,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateCaseReviewFile(
	ctx context.Context,
	exec Executor,
	caseReviewId uuid.UUID,
	status models.UpdateAiCaseReview,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_AI_CASE_REVIEWS).
		Set("status", status.Status.String()).
		Where(squirrel.Eq{"id": caseReviewId})

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) GetCaseReviewFile(
	ctx context.Context,
	exec Executor,
	aiCaseReviewId uuid.UUID,
) (models.AiCaseReview, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.AiCaseReview{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.AiCaseReviewFields...).
		From(dbmodels.TABLE_AI_CASE_REVIEWS).
		Where(squirrel.Eq{"id": aiCaseReviewId})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptAiCaseReview)
}

func (repo *MarbleDbRepository) ListCaseReviewFiles(
	ctx context.Context,
	exec Executor,
	caseId uuid.UUID,
) ([]models.AiCaseReview, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.AiCaseReviewFields...).
		From(dbmodels.TABLE_AI_CASE_REVIEWS).
		Where(squirrel.Eq{
			"case_id": caseId,
			"status":  models.AiCaseReviewStatusCompleted.String(),
		}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		func(dbModel dbmodels.AiCaseReview) (models.AiCaseReview, error) {
			return dbmodels.AdaptAiCaseReview(dbModel)
		},
	)
}

func (repo *MarbleDbRepository) ListAllCaseReviewFiles(
	ctx context.Context,
	exec Executor,
	caseId uuid.UUID,
) ([]models.AiCaseReview, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.AiCaseReviewFields...).
		From(dbmodels.TABLE_AI_CASE_REVIEWS).
		Where(squirrel.Eq{"case_id": caseId}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		func(dbModel dbmodels.AiCaseReview) (models.AiCaseReview, error) {
			return dbmodels.AdaptAiCaseReview(dbModel)
		},
	)
}

func (repo *MarbleDbRepository) CountAiCaseReviewsByOrg(
	ctx context.Context,
	exec Executor,
	orgIds []string,
	from, to time.Time,
) (map[string]int, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select("c.org_id, count(*) as count").
		From(dbmodels.TABLE_AI_CASE_REVIEWS + " AS acr").
		Join(dbmodels.TABLE_CASES + " AS c ON acr.case_id = c.id").
		Where(squirrel.Eq{"c.org_id": orgIds}).
		Where(squirrel.GtOrEq{"acr.created_at": from}).
		Where(squirrel.Lt{"acr.created_at": to}).
		GroupBy("c.org_id")

	return countByHelper(ctx, exec, query, orgIds)
}

// For now, update the feedback for the most recent completed case review.
func (repo *MarbleDbRepository) UpdateAiCaseReviewFeedback(
	ctx context.Context,
	exec Executor,
	reviewId uuid.UUID,
	feedback models.AiCaseReviewFeedback,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(ctx, exec, NewQueryBuilder().
		Update(dbmodels.TABLE_AI_CASE_REVIEWS).
		Set("reaction", feedback.Reaction).
		Where(
			squirrel.Eq{
				"id": reviewId,
			},
		),
	)
	return err
}

func (repo *MarbleDbRepository) GetCaseReviewById(
	ctx context.Context,
	exec Executor,
	reviewId uuid.UUID,
) (models.AiCaseReview, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.AiCaseReview{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.AiCaseReviewFields...).
			From(dbmodels.TABLE_AI_CASE_REVIEWS).
			Where(squirrel.Eq{"id": reviewId}),
		dbmodels.AdaptAiCaseReview,
	)
}
