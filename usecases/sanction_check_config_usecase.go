package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type SanctionCheckConfigRepository interface {
	GetSanctionCheckConfig(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (models.SanctionCheckConfig, error)
	UpdateSanctionCheckConfig(ctx context.Context, exec repositories.Executor,
		scenarioIterationId string, sanctionCheckConfig models.SanctionCheckConfig) (models.SanctionCheckConfig, error)
}

// TODO: Will we have a usecase for sanction checks?
