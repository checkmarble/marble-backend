package main

import (
	"flag"
	"log"
	"log/slog"

	"github.com/checkmarble/marble-backend/cmd"
	"github.com/checkmarble/marble-backend/utils"
)

func main() {
	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	shouldRunScheduleScenarios := flag.Bool("scheduler", false, "Run schedule scenarios")
	shouldRunExecuteScheduledScenarios := flag.Bool("scheduled-executer", false, "Run execute scheduled scenarios")
	shouldRunDataIngestion := flag.Bool("data-ingestion", false, "Run data ingestion")
	shouldRunSendPendingWebhookEvents := flag.Bool("send-pending-webhook-events", false, "Send pending webhook events")
	shouldRunScheduler := flag.Bool("cron-scheduler", false, "Run scheduler for cron jobs")
	flag.Parse()
	logger := utils.NewLogger("text")
	logger.Info("Flags",
		slog.Bool("shouldRunMigrations", *shouldRunMigrations),
		slog.Bool("shouldRunServer", *shouldRunServer),
		slog.Bool("shouldRunScheduledScenarios", *shouldRunScheduleScenarios),
		slog.Bool("shouldRunDataIngestion", *shouldRunDataIngestion),
		slog.Bool("shouldRunScheduler", *shouldRunScheduler),
		slog.Bool("shouldRunSendPendingWebhookEvents", *shouldRunSendPendingWebhookEvents),
	)

	if *shouldRunMigrations {
		if err := cmd.RunMigrations(); err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunServer {
		err := cmd.RunServer()
		if err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunScheduleScenarios {
		// TODOl: eventually, remove this entrypoint completely
		logger.Info("The entrypoint \"scheduler\" is deprecated, its functionality has been merged into the \"scheduled-executer\" entrypoint")
	}

	if *shouldRunExecuteScheduledScenarios {
		err := cmd.RunScheduledExecuter()
		if err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunDataIngestion {
		err := cmd.RunBatchIngestion()
		if err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunSendPendingWebhookEvents {
		err := cmd.RunSendPendingWebhookEvents()
		if err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunScheduler {
		err := cmd.RunJobScheduler()
		if err != nil {
			log.Fatal(err)
		}
	}
}
