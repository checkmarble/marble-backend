package cmd

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/checkmarble/marble-backend/api"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

func RunAnalyticsServer(config CompiledConfig) error {
	appName := "marble-analytics"

	logger := utils.NewLogger(utils.GetEnv("LOGGING_FORMAT", "text"))
	ctx := utils.StoreLoggerInContext(context.Background(), logger)

	port := utils.GetEnv("ANALYTICS_PORT", "")
	if port == "" {
		port = utils.GetEnv("PORT", "")
	}

	if port == "" {
		log.Fatalf("ANALYTICS_PORT or PORT environment variable is required")
	}

	apiConfig := api.Configuration{
		Env:                 utils.GetEnv("ENV", "development"),
		AppName:             appName,
		Port:                port,
		RequestLoggingLevel: utils.GetEnv("REQUEST_LOGGING_LEVEL", "all"),
		AnalyticsTimeout:    utils.GetEnvDuration("ANALYTICS_TIMEOUT", 15*time.Second),
	}

	pgConfig := infra.PgConfig{
		ConnectionString:   utils.GetEnv("PG_CONNECTION_STRING", ""),
		Database:           utils.GetEnv("PG_DATABASE", "marble"),
		Hostname:           utils.GetEnv("PG_HOSTNAME", ""),
		Password:           utils.GetEnv("PG_PASSWORD", ""),
		Port:               utils.GetEnv("PG_PORT", "5432"),
		User:               utils.GetEnv("PG_USER", ""),
		MaxPoolConnections: utils.GetEnv("PG_MAX_POOL_SIZE", infra.DEFAULT_MAX_CONNECTIONS),
		ClientDbConfigFile: utils.GetEnv("CLIENT_DB_CONFIG_FILE", ""),
		SslMode:            utils.GetEnv("PG_SSL_MODE", "prefer"),
	}

	serverConfig := struct {
		jwtSigningKey      string
		jwtSigningKeyFile  string
		analyticsBucketUrl string
	}{
		jwtSigningKey:      utils.GetEnv("AUTHENTICATION_JWT_SIGNING_KEY", ""),
		jwtSigningKeyFile:  utils.GetEnv("AUTHENTICATION_JWT_SIGNING_KEY_FILE", ""),
		analyticsBucketUrl: utils.GetEnv("ANALYTICS_BUCKET_URL", ""),
	}

	licenseConfig := models.LicenseConfiguration{
		LicenseKey:             utils.GetEnv("LICENSE_KEY", ""),
		KillIfReadLicenseError: utils.GetEnv("KILL_IF_READ_LICENSE_ERROR", false),
	}

	pool, err := infra.NewPostgresConnectionPool(ctx, appName, pgConfig.GetConnectionString(), nil, pgConfig.MaxPoolConnections)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
	}

	clientDbConfig, err := infra.ParseClientDbConfig(pgConfig.ClientDbConfigFile)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	analyticsConfig, err := infra.InitAnalyticsConfig(serverConfig.analyticsBucketUrl)
	if err != nil {
		return err
	}

	marbleJwtSigningKey := infra.ReadParseOrGenerateSigningKey(ctx, serverConfig.jwtSigningKey, serverConfig.jwtSigningKeyFile)
	license := infra.VerifyLicense(licenseConfig, "")

	repositories := repositories.NewRepositories(
		pool,
		infra.GcpConfig{},
		repositories.WithClientDbConfig(clientDbConfig),
	)

	deps := api.InitDependencies(ctx, apiConfig, pool, marbleJwtSigningKey)

	uc := usecases.NewUsecases(repositories,
		usecases.WithAppName(appName),
		usecases.WithApiVersion(config.Version),
		usecases.WithLicense(license),
		usecases.WithAnalyticsConfig(analyticsConfig),
	)

	router := api.InitRouterMiddlewares(ctx, apiConfig, true, nil, infra.TelemetryRessources{})
	server := api.NewAnalyticsServer(router, apiConfig, uc, deps.Authentication)

	notify, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.InfoContext(ctx, "starting server", slog.String("version", config.Version), slog.String("port", apiConfig.Port))

		err := server.ListenAndServe()

		if !errors.Is(err, http.ErrServerClosed) {
			utils.LogAndReportSentryError(ctx, errors.Wrap(err, "Error while serving the app"))
		}

		logger.InfoContext(ctx, "server returned")
	}()

	<-notify.Done()
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		utils.LogAndReportSentryError(
			ctx,
			errors.Wrap(err, "Error while shutting down the server"),
		)
		return err
	}

	return err
}
