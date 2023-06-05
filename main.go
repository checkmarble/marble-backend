package main

import (
	"context"
	"flag"
	"log"
	"marble/marble-backend/api"
	"marble/marble-backend/app"
	"marble/marble-backend/infra"
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases"
	"marble/marble-backend/utils"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/exp/slog"
)

func runServer(config usecases.Configuration, pgRepository *pg_repository.PGRepository, marbleConnectionPool *pgxpool.Pool, port string, env string, logger *slog.Logger) {
	ctx := context.Background()

	devEnv := env == "DEV"

	corsAllowLocalhost := devEnv

	marbleJwtSigningKey := infra.MustParseSigningKey(utils.GetRequiredStringEnv("AUTHENTICATION_JWT_SIGNING_KEY"))

	app, _ := app.New(pgRepository)
	repositories := repositories.NewRepositories(
		marbleJwtSigningKey,
		infra.IntializeFirebase(ctx),
		pgRepository,
		marbleConnectionPool,
		map[models.DatabaseName]string{
			"zorg": pg_repository.PGConfig{
				Hostname: "localhost",
				Port:     "5432",
				User:     "zorg",
				Password: "very-secret",
				Database: "marbleclient_zorg",
			}.GetConnectionString(env),
		},
	)

	usecases := usecases.Usecases{
		Repositories: *repositories,
		Config:       config,
	}

	////////////////////////////////////////////////////////////
	// Seed the database
	////////////////////////////////////////////////////////////

	seedUsecase := usecases.NewSeedUseCase()

	marbleAdminEmail, _ := os.LookupEnv("MARBLE_ADMIN_EMAIL")
	if marbleAdminEmail != "" {
		err := seedUsecase.SeedMarbleAdmins(marbleAdminEmail)
		if err != nil {
			panic(err)
		}
	}

	if devEnv || env == "staging" {
		zorgOrganizationId := "13617a88-56f5-4baa-8d11-ce102f7da907"
		err := seedUsecase.SeedZorgOrganization(zorgOrganizationId)
		if err != nil {
			panic(err)
		}
		// TODO: this seed must occur for every new organization
		// pgRepository.Seed(zorgOrganizationId)
	}

	api, _ := api.New(ctx, port, app, usecases, logger, corsAllowLocalhost)

	////////////////////////////////////////////////////////////
	// Start serving the app
	////////////////////////////////////////////////////////////

	// Intercept SIGxxx signals
	notify, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("starting server on port %v\n", port)
		if err := api.ListenAndServe(); err != nil {
			logger.ErrorCtx(ctx, "error serving the app: \n"+err.Error())
		}
		logger.InfoCtx(ctx, "server returned")
	}()

	// Block until we receive our signal.
	<-notify.Done()
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	api.Shutdown(shutdownCtx)
}

func main() {

	var (
		env        = utils.GetStringEnv("ENV", "DEV")
		port       = utils.GetRequiredStringEnv("PORT")
		pgPort     = utils.GetStringEnv("PG_PORT", "5432")
		pgHostname = utils.GetRequiredStringEnv("PG_HOSTNAME")
		pgUser     = utils.GetRequiredStringEnv("PG_USER")
		pgPassword = utils.GetRequiredStringEnv("PG_PASSWORD")
		pgDatabase = "marble"
		config     = usecases.Configuration{
			TokenLifetimeMinute: utils.GetIntEnv("TOKEN_LIFETIME_MINUTE", 30),
		}
	)

	////////////////////////////////////////////////////////////
	// Setup dependencies
	////////////////////////////////////////////////////////////

	var logger *slog.Logger
	if env == "DEV" {
		textHandler := slog.HandlerOptions{ReplaceAttr: utils.LoggerAttributeReplacer}.NewTextHandler(os.Stderr)
		logger = slog.New(textHandler)
	} else {
		jsonHandler := slog.HandlerOptions{ReplaceAttr: utils.LoggerAttributeReplacer}.NewJSONHandler(os.Stderr)
		logger = slog.New(jsonHandler)
	}

	pgConfig := pg_repository.PGConfig{
		Hostname: pgHostname,
		Port:     pgPort,
		User:     pgUser,
		Password: pgPassword,
		Database: pgDatabase,
	}

	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	shouldWipeDb := flag.Bool("wipe", false, "Truncate db tables")
	flag.Parse()
	logger.DebugCtx(context.Background(), "shouldRunMigrations", *shouldRunMigrations, "shouldRunServer", *shouldRunServer)

	if *shouldWipeDb {
		pg_repository.WipeDb(env, pgConfig, logger)
	}
	if *shouldRunMigrations {
		pg_repository.RunMigrations(env, pgConfig, logger)
	}
	if *shouldRunServer {

		connectionString := pgConfig.GetConnectionString(env)
		marbleConnectionPool, err := infra.NewPostgresConnectionPool(connectionString)
		if err != nil {
			log.Fatal("error creating postgres connection to marble database", err.Error())
		}

		pgRepository, err := pg_repository.New(marbleConnectionPool)
		if err != nil {
			logger.Error("error creating pg repository:\n", err.Error())
			return
		}
		runServer(config, pgRepository, marbleConnectionPool, port, env, logger)
	}
}
