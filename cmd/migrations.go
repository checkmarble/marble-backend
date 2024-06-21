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
		Database:            "marble",
		DbConnectWithSocket: utils.GetEnv("PG_CONNECT_WITH_SOCKET", false),
		Hostname:            utils.GetRequiredEnv[string]("PG_HOSTNAME"),
		Password:            utils.GetRequiredEnv[string]("PG_PASSWORD"),
		Port:                utils.GetEnv("PG_PORT", "5432"),
		User:                utils.GetRequiredEnv[string]("PG_USER"),
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
