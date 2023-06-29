package main

import (
	"context"
	"flag"
	"log"
	"marble/marble-backend/infra"
	"marble/marble-backend/jobs"
	"marble/marble-backend/pg_repository"
	"marble/marble-backend/server"
	"marble/marble-backend/utils"
	"os"

	"golang.org/x/exp/slog"
)

func main() {

	config := server.InitConfig()

	////////////////////////////////////////////////////////////
	// Setup dependencies
	////////////////////////////////////////////////////////////

	var logger *slog.Logger
	if config.Env == "DEV" {
		textHandler := slog.HandlerOptions{ReplaceAttr: utils.LoggerAttributeReplacer}.NewTextHandler(os.Stderr)
		logger = slog.New(textHandler)
	} else {
		jsonHandler := slog.HandlerOptions{ReplaceAttr: utils.LoggerAttributeReplacer}.NewJSONHandler(os.Stderr)
		logger = slog.New(jsonHandler)
	}

	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	shouldRunScheduledScenarios := flag.Bool("scheduler", false, "Run scheduled scenarios")
	flag.Parse()
	logger.DebugCtx(context.Background(), "shouldRunMigrations", *shouldRunMigrations, "shouldRunServer", *shouldRunServer)

	if *shouldRunMigrations {
		pg_repository.RunMigrations(config.Env, config.PGConfig, logger)
	}
	if *shouldRunServer {
		serv, err := server.NewServer(config, logger)
		if err != nil {
			logger.Error("Error couldn't start the server: ", err)
			return
		}
		serv.Run()
	}

	if *shouldRunScheduledScenarios {
		connectionString := config.PGConfig.GetConnectionString(config.Env)
		marbleConnectionPool, err := infra.NewPostgresConnectionPool(connectionString)
		if err != nil {
			log.Fatal("error creating postgres connection to marble database", err.Error())
		}

		pgRepository, err := pg_repository.New(marbleConnectionPool)
		if err != nil {
			logger.Error("error creating pg repository:\n", err.Error())
			return
		}
		jobs.RunScheduledBatches(config.GlobalConfiguration, pgRepository, marbleConnectionPool, logger)
	}
}
