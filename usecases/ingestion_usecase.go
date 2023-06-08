package usecases

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"

	"golang.org/x/exp/slog"
)

type IngestionUseCase struct {
	orgTransactionFactory organization.OrgTransactionFactory
	ingestionRepository   repositories.IngestionRepository
}

func (usecase *IngestionUseCase) IngestObject(organizationId string, payload models.Payload, table models.Table, logger *slog.Logger) error {

	return usecase.orgTransactionFactory.TransactionInOrgSchema(organizationId, func(tx repositories.Transaction) error {
		return usecase.ingestionRepository.IngestObject(tx, payload, table, logger)
	})
}
