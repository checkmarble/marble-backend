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
	flag.Parse()
	logger := utils.NewLogger("text")
	logger.Info("Flags",
		slog.Bool("shouldRunMigrations", *shouldRunMigrations),
		slog.Bool("shouldRunServer", *shouldRunServer),
		slog.Bool("shouldRunWorker", *shouldRunWorker),
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
		err := cmd.RunTaskQueue(apiVersion)
		if err != nil {
			log.Fatal(err)
		}
	}
}
