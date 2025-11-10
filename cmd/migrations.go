package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

func RunMigrations(apiVersion string, migrateDownTo *int64) error {
	pgConfig := infra.PgConfig{
		ConnectionString: utils.GetEnv("PG_CONNECTION_STRING", ""),
		Database:         utils.GetEnv("PG_DATABASE", "marble"),
		Hostname:         utils.GetEnv("PG_HOSTNAME", ""),
		Password:         utils.GetEnv("PG_PASSWORD", ""),
		Port:             utils.GetEnv("PG_PORT", "5432"),
		User:             utils.GetEnv("PG_USER", ""),
		SslMode:          utils.GetEnv("PG_SSL_MODE", "prefer"),
		ImpersonateRole:  utils.GetEnv("PG_IMPERSONATE_ROLE", ""),
	}
	if pgConfig.ConnectionString != "" {
		if u, err := url.Parse(pgConfig.ConnectionString); err != nil || !u.IsAbs() {
			switch err {
			case nil:
				return errors.New("invalid database connection string")
			default:
				return errors.Wrap(err, "invalid database connection string")
			}
		}
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
