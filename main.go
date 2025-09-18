package main

import (
	"flag"
	"log"
	"log/slog"

	"github.com/checkmarble/marble-backend/cmd"
	"github.com/checkmarble/marble-backend/utils"
)

// Static variable set at compilation-time through linker flags.
//
//	$ go build -ldflags '-X main.apiVersion=v0.10.0 -X ...' .
var (
	apiVersion      string = "dev"
	segmentWriteKey string = ""
)

var compiledConfig = cmd.CompiledConfig{
	Version:         apiVersion,
	SegmentWriteKey: segmentWriteKey,
}

func main() {
	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	shouldRunWorker := flag.Bool("worker", false, "Run workers on the task queues")
	shouldRunAnalyticsServer := flag.Bool("analytics", false, "Run analytics server")

	// DEVELOPMENT-ONLY: those flags are used to help debugging and cannot be used in production
	var (
		workerOnly     *string = utils.Ptr("")
		workerOnlyArgs *string = utils.Ptr("")
	)

	if apiVersion == "dev" {
		workerOnly = flag.String("worker-only", "", "only run a specific job to completion")
		workerOnlyArgs = flag.String("worker-args", "", "JSON-encoded arguments to the worker")
	}

	flag.Parse()

	if !*shouldRunWorker && (*workerOnly != "" || *workerOnlyArgs != "") {
		log.Fatal("-worker-only and -worker-args can only be used when running the worker")
	}

	logger := utils.NewLogger("text")
	logger.Info("Flags",
		slog.Bool("shouldRunMigrations", *shouldRunMigrations),
		slog.Bool("shouldRunServer", *shouldRunServer),
		slog.Bool("shouldRunWorker", *shouldRunWorker),
		slog.Bool("shouldRunAnalyticsServer", *shouldRunAnalyticsServer),
	)

	if *shouldRunMigrations {
		if err := cmd.RunMigrations(apiVersion); err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunServer {
		err := cmd.RunServer(compiledConfig)
		if err != nil {
			log.Fatal(err)
		}
	}

	if *shouldRunWorker {
		err := cmd.RunTaskQueue(apiVersion, *workerOnly, *workerOnlyArgs)
		if err != nil {
			log.Fatal(err)
		}
	}
}
