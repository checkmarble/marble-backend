package main

import (
	"flag"
	"log"
	"log/slog"

	"github.com/checkmarble/marble-backend/cmd"
	"github.com/checkmarble/marble-backend/utils"
)

var apiVersion string = "dev"

func main() {
	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	shouldRunScheduleScenarios := flag.Bool("scheduler", false, "Run schedule scenarios")
	shouldRunExecuteScheduledScenarios := flag.Bool("scheduled-executer", false, "Run execute scheduled scenarios")
	shouldRunDataIngestion := flag.Bool("data-ingestion", false, "Run data ingestion")
	shouldRunSendPendingWebhookEvents := flag.Bool("send-pending-webhook-events", false, "Send pending webhook events")
	shouldRunScheduler := flag.Bool("cron-scheduler", false, "Run scheduler for cron jobs")
	shouldRunWorker := flag.Bool("worker", false, "Run workers on the task queues")
	flag.Parse()
	logger := utils.NewLogger("text")
	logger.Info("Flags",
		slog.Bool("shouldRunMigrations", *shouldRunMigrations),
		slog.Bool("shouldRunServer", *shouldRunServer),
		slog.Bool("shouldRunScheduledScenarios", *shouldRunScheduleScenarios),
		slog.Bool("shouldRunDataIngestion", *shouldRunDataIngestion),
		slog.Bool("shouldRunScheduler", *shouldRunScheduler),
		slog.Bool("shouldRunSendPendingWebhookEvents", *shouldRunSendPendingWebhookEvents),
		slog.Bool("shouldRunWorker", *shouldRunWorker),
	)

	if *shouldRunMigrations {
		if err := cmd.RunMigrations(apiVersion); err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunServer {
		err := cmd.RunServer(apiVersion)
		if err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunScheduleScenarios {
		// TODO: eventually, remove this entrypoint completely
		logger.Info("The entrypoint \"scheduler\" is deprecated, its functionality has been merged into the \"scheduled-executer\" entrypoint")
	}

	if *shouldRunExecuteScheduledScenarios {
		err := cmd.RunScheduledExecuter(apiVersion)
		if err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunDataIngestion {
		err := cmd.RunBatchIngestion(apiVersion)
		if err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunSendPendingWebhookEvents {
		err := cmd.RunSendPendingWebhookEvents(apiVersion)
		if err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunScheduler {
		// TODO: deprecated in favor of the task queue worker, which now runs the cron jobs. Will be removed eventually.
		logger.Info("The entrypoint \"cron-scheduler\" is deprecated, its functionality has been merged into the \"worker\" entrypoint")
		err := cmd.RunTaskQueue(apiVersion)
		if err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunWorker {
		err := cmd.RunTaskQueue(apiVersion)
		if err != nil {
			log.Fatal(err)
		}
	}
}
