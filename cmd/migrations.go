package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
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

	// Run a non-blocking basic http server to respond to Cloud Run http probes, to respect the Cloud Run
	// contract. It is stopped as soon as the migrations are done, so that containers running alongside the
	// migration job (such as the Cloud SQL auth proxy) can use it as a signal to shut down.
	if probePort := utils.GetEnv("CLOUD_RUN_PROBE_PORT", ""); probePort != "" {
		stopProbeServer := runMigrationProbeServer(ctx, probePort)
		defer stopProbeServer()
	}

	migrater := repositories.NewMigrater(pgConfig)

	if err := migrater.Run(ctx, migrateDownTo); err != nil {
		logger.ErrorContext(ctx, fmt.Sprintf("error running migrations: %v", err))
		return err
	}

	return nil
}

// Serves the liveness probe endpoints for the duration of the migrations, and returns a function
// shutting the server down.
func runMigrationProbeServer(ctx context.Context, port string) func() {
	logger := utils.LoggerFromContext(ctx)

	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})
	r.GET("/liveness", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.ErrorContext(ctx, fmt.Sprintf("error running migration probe server: %v", err))
		}
	}()

	return func() {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("error shutting down migration probe server: %v", err))
		}
	}
}
