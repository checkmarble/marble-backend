package jobs

import (
	"context"
	"crypto/rsa"
	"marble/marble-backend/infra"
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases"
	"marble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/exp/slog"
)

func RunScheduledBatches(configuration models.GlobalConfiguration, pgRepository *pg_repository.PGRepository, marbleConnectionPool *pgxpool.Pool, logger *slog.Logger) {
	ctx := utils.StoreLoggerInContext(context.Background(), logger)

	repositories, err := repositories.NewRepositories(
		configuration,
		rsa.PrivateKey{},
		infra.IntializeFirebase(ctx),
		pgRepository,
		marbleConnectionPool,
		logger,
	)
	if err != nil {
		panic(err)
	}

	usecases := usecases.Usecases{
		Repositories:  *repositories,
		Configuration: configuration,
	}

	ExecuteAllScheduledScenarios(ctx, usecases)
}
