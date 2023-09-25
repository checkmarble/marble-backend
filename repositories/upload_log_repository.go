package repositories

import (
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type UploadLogRepository interface {
	CreateUploadLog(tx Transaction, log models.UploadLog) error
	UploadLogById(tx Transaction, id string) (models.UploadLog, error)
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

func (repo *UploadLogRepositoryImpl) UploadLogById(tx Transaction, id string) (models.UploadLog, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	uploadLog, err := SqlToModel(pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectUploadLogColumn...).
			From(dbmodels.TABLE_UPLOAD_LOGS).
			Where(squirrel.Eq{"id": id}),
		func(db dbmodels.DBUploadLog) models.UploadLog {
			return dbmodels.AdaptUploadLog(db)
		},
	)

	if err != nil {
		return models.UploadLog{}, err
	}

	return uploadLog, err
}
