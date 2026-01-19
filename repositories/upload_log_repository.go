package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type UploadLogRepository interface {
	CreateUploadLog(ctx context.Context, exec Executor, log models.UploadLog) error
	UpdateUploadLogStatus(ctx context.Context, exec Executor, input models.UpdateUploadLogStatusInput) (executed bool, err error)
	UploadLogById(ctx context.Context, exec Executor, id string) (models.UploadLog, error)
	AllUploadLogsByTable(ctx context.Context, exec Executor, organizationId uuid.UUID,
		tableName string) ([]models.UploadLog, error)
}

type UploadLogRepositoryImpl struct{}

func (repo *UploadLogRepositoryImpl) CreateUploadLog(ctx context.Context, exec Executor, log models.UploadLog) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_UPLOAD_LOGS).
			Columns(
				"id",
				"org_id",
				"user_id",
				"file_name",
				"table_name",
				"status",
				"started_at",
				"finished_at",
				"lines_processed",
			).
			Values(
				log.Id,
				log.OrganizationId,
				log.UserId,
				log.FileName,
				log.TableName,
				log.UploadStatus,
				log.StartedAt,
				log.FinishedAt,
				log.LinesProcessed,
			),
	)
	return err
}

func (repo *UploadLogRepositoryImpl) UpdateUploadLogStatus(
	ctx context.Context,
	exec Executor,
	input models.UpdateUploadLogStatusInput,
) (executed bool, err error) {
	// uses optimistic locking to prevent inconsistent updates of the status
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}

	updateRequest := NewQueryBuilder().Update(dbmodels.TABLE_UPLOAD_LOGS)

	if input.UploadStatus != "" {
		updateRequest = updateRequest.Set("status", input.UploadStatus)
	}
	if input.FinishedAt != nil {
		updateRequest = updateRequest.Set("finished_at", *input.FinishedAt)
	}
	if input.NumRowsIngested != nil {
		updateRequest = updateRequest.Set("num_rows_ingested", *input.NumRowsIngested)
	}
	if input.Error != nil {
		updateRequest = updateRequest.Set("error", *input.Error)
	}

	updateRequest = updateRequest.
		Where("id = ?", input.Id).
		Where("status = ?", input.CurrentUploadStatusCondition)

	sql, args, err := updateRequest.ToSql()
	if err != nil {
		return false, err
	}

	tag, err := exec.Exec(ctx, sql, args...)
	if err != nil {
		return false, err
	}

	return tag.RowsAffected() > 0, nil
}

func (repo *UploadLogRepositoryImpl) UploadLogById(ctx context.Context, exec Executor, id string) (models.UploadLog, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.UploadLog{}, err
	}

	uploadLog, err := SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectUploadLogColumn...).
			From(dbmodels.TABLE_UPLOAD_LOGS).
			Where(squirrel.Eq{"id": id}),
		dbmodels.AdaptUploadLog,
	)
	if err != nil {
		return models.UploadLog{}, err
	}

	return uploadLog, err
}

func (repo *UploadLogRepositoryImpl) AllUploadLogsByTable(
	ctx context.Context,
	exec Executor,
	organizationId uuid.UUID,
	tableName string,
) ([]models.UploadLog, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectUploadLogColumn...).
			From(dbmodels.TABLE_UPLOAD_LOGS).
			Where(squirrel.Eq{"org_id": organizationId}).
			Where(squirrel.Eq{"table_name": tableName}).
			OrderBy("started_at DESC"),
		dbmodels.AdaptUploadLog,
	)
}
