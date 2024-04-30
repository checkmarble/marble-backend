package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

type TransferCheckEnrichmentRepository struct{}

func NewTransferCheckEnrichmentRepository() TransferCheckEnrichmentRepository {
	return TransferCheckEnrichmentRepository{}
}

func (r TransferCheckEnrichmentRepository) GetIPCountry(ctx context.Context, ip string) (string, error) {
	return "FR", nil
}

func (r TransferCheckEnrichmentRepository) GetIPType(ctx context.Context, ip string) (string, error) {
	return models.RegularIP, nil
}

func (r TransferCheckEnrichmentRepository) GetSenderBicRiskLevel(ctx context.Context, bic string) (string, error) {
	return models.RegularSender, nil
}
