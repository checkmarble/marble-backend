package repositories

import (
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type UploadLogRepository interface {
	CreateUploadLog(tx Transaction, log models.UploadLog) error
	UpdateUploadLog(tx Transaction, input models.UpdateUploadLogInput) error
	UploadLogById(tx Transaction, id string) (models.UploadLog, error)
	AllUploadLogsByStatus(tx Transaction, status models.UploadStatus) ([]models.UploadLog, error)
}

type UploadLogRepositoryImpl struct {
	transactionFactory TransactionFactoryPosgresql
}

func (repo *UploadLogRepositoryImpl) CreateUploadLog(tx Transaction, log models.UploadLog) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_UPLOAD_LOGS).
			Columns(
				"id",
				"org_id",
				"user_id",
				"file_name",
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
				log.UploadStatus,
				log.StartedAt,
				log.FinishedAt,
				log.LinesProcessed,
			),
	)
	return err
}

func (repo *UploadLogRepositoryImpl) UpdateUploadLog(tx Transaction, input models.UpdateUploadLogInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	var updateRequest = NewQueryBuilder().Update(dbmodels.TABLE_UPLOAD_LOGS)

	if input.UploadStatus != "" {
		updateRequest = updateRequest.Set("status", input.UploadStatus)
	}
	if input.FinishedAt != nil {
		updateRequest = updateRequest.Set("finished_at", *input.FinishedAt)
	}
	updateRequest = updateRequest.Where("id = ?", input.Id)

	_, err := pgTx.ExecBuilder(updateRequest)
	return err
}

func (repo *UploadLogRepositoryImpl) UploadLogById(tx Transaction, id string) (models.UploadLog, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	uploadLog, err := SqlToModel(pgTx,
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

func (repo *UploadLogRepositoryImpl) AllUploadLogsByStatus(tx Transaction, status models.UploadStatus) ([]models.UploadLog, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectUploadLogColumn...).
			From(dbmodels.TABLE_UPLOAD_LOGS).
			Where(squirrel.Eq{"status": status}).
			OrderBy("started_at"),
		dbmodels.AdaptUploadLog,
	)
}
