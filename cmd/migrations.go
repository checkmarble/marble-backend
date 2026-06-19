package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
)

func RunMigrations(apiVersion string, migrateDownTo *int64) error {
	pgConfig, err := infra.NewPgConfig()
	if err != nil {
		return fmt.Errorf("load postgres config for migrations: %w", err)
	}

	logger := utils.NewLogger(utils.GetEnv("LOGGING_FORMAT", "text"))
	ctx := utils.StoreLoggerInContext(context.Background(), logger)

	logger.InfoContext(ctx, "starting migrator", slog.String("version", apiVersion))

	migrater := repositories.NewMigrater(pgConfig)

	if err := migrater.Run(ctx, migrateDownTo); err != nil {
		logger.ErrorContext(ctx, fmt.Sprintf("error running migrations: %v", err))
		return err
	}

	return nil
}
