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
	shouldRunSendPendingWebhooks := flag.Bool("send-pending-webhooks", false, "Send pending webhooks")
	shouldRunScheduler := flag.Bool("cron-scheduler", false, "Run scheduler for cron jobs")
	flag.Parse()
	utils.NewLogger("text").Info("Flags",
		slog.Bool("shouldRunMigrations", *shouldRunMigrations),
		slog.Bool("shouldRunServer", *shouldRunServer),
		slog.Bool("shouldRunScheduledScenarios", *shouldRunScheduleScenarios),
		slog.Bool("shouldRunDataIngestion", *shouldRunDataIngestion),
		slog.Bool("shouldRunScheduler", *shouldRunScheduler),
		slog.Bool("shouldRunSendPendingWebhooks", *shouldRunSendPendingWebhooks),
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
		err := cmd.RunScheduleScenarios()
		if err != nil {
			log.Fatal(err)
		}
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

	if *shouldRunSendPendingWebhooks {
		err := cmd.RunSendPendingWebhooks()
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
