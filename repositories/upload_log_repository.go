package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type UploadLogRepository interface {
	CreateUploadLog(ctx context.Context, exec Executor, log models.UploadLog) error
	UpdateUploadLogStatus(ctx context.Context, exec Executor, input models.UpdateUploadLogStatusInput) (executed bool, err error)
	SaveUploadLogCheckpoint(ctx context.Context, exec Executor, id uuid.UUID, byteOffset int64, rowsIngested int) error
	UploadLogById(ctx context.Context, exec Executor, id uuid.UUID) (models.UploadLog, error)
	AllUploadLogsByTable(ctx context.Context, exec Executor, organizationId uuid.UUID,
		tableName string) ([]models.UploadLog, error)
	ListUploadLogs(ctx context.Context, exec Executor, organizationId uuid.UUID,
		tableName string, filters models.UploadLogFilters, pagination models.PaginationAndSorting) ([]models.UploadLog, error)
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
				"input_error",
				"error",
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
				log.InputError,
				log.Error,
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
	if input.InputError != nil {
		updateRequest = updateRequest.Set("input_error", *input.InputError)
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

// SaveUploadLogCheckpoint records how far into the CSV the ingestion got, so a later attempt can
// resume from there rather than restarting the file.
func (repo *UploadLogRepositoryImpl) SaveUploadLogCheckpoint(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
	byteOffset int64,
	rowsIngested int,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	return ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Update(dbmodels.TABLE_UPLOAD_LOGS).
			Set("byte_offset", byteOffset).
			Set("num_rows_ingested", rowsIngested).
			Where(squirrel.Eq{"id": id}).
			Where(squirrel.Eq{"status": models.UploadProcessing}).
			// Never let the offset move backwards. River normally runs a single attempt of a job at a
			// time, but that has edges — its rescuer requeues at the Timeout boundary while the previous
			// attempt is still unwinding, and the single-job CLI path can be pointed at a running upload
			// log — and a stale attempt rewinding the checkpoint would re-ingest everything past it.
			Where(squirrel.LtOrEq{"byte_offset": byteOffset}),
	)
}

func (repo *UploadLogRepositoryImpl) UploadLogById(ctx context.Context, exec Executor, id uuid.UUID) (models.UploadLog, error) {
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

func (repo *UploadLogRepositoryImpl) ListUploadLogs(
	ctx context.Context,
	exec Executor,
	organizationId uuid.UUID,
	tableName string,
	filters models.UploadLogFilters,
	pagination models.PaginationAndSorting,
) ([]models.UploadLog, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectUploadLogColumn...).
		From(dbmodels.TABLE_UPLOAD_LOGS).
		Where(squirrel.Eq{"org_id": organizationId}).
		Where(squirrel.Eq{"table_name": tableName}).
		OrderBy("started_at DESC, id DESC").
		Limit(uint64(pagination.Limit))

	if filters.Status != nil {
		query = query.Where("status = ?", *filters.Status)
	}

	if pagination.OffsetId != "" {
		offsetId, err := uuid.Parse(pagination.OffsetId)
		if err != nil {
			return nil, errors.Wrap(err, "provided upload log offset ID was not a UUID")
		}
		offsetLog, err := repo.UploadLogById(ctx, exec, offsetId)
		if err != nil {
			return nil, err
		}
		query = query.Where("(started_at, id) < (?, ?)", offsetLog.StartedAt, offsetLog.Id)
	}

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptUploadLog,
	)
}
