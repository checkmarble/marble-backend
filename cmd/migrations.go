package cmd

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
)

func RunMigrations() error {
	pgConfig := infra.PgConfig{
		ConnectionString: utils.GetEnv("PG_CONNECTION_STRING", ""),
		Database:         "marble",
		Hostname:         utils.GetEnv("PG_HOSTNAME", ""),
		Password:         utils.GetEnv("PG_PASSWORD", ""),
		Port:             utils.GetEnv("PG_PORT", "5432"),
		User:             utils.GetEnv("PG_USER", ""),
		SslMode:          utils.GetEnv("PG_SSL_MODE", "prefer"),
	}

	logger := utils.NewLogger(utils.GetEnv("LOGGING_FORMAT", "text"))
	ctx := utils.StoreLoggerInContext(context.Background(), logger)

	migrater := repositories.NewMigrater(pgConfig)
	if err := migrater.Run(ctx); err != nil {
		logger.ErrorContext(ctx, fmt.Sprintf("error running migrations: %v", err))
		return err
	}

	return nil
}
