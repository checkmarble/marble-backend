package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2/google"
)

func NewLogger(format string) *slog.Logger {
	var logger *slog.Logger
	if !slices.Contains([]string{"text", "json", "gcp"}, format) {
		fmt.Printf("invalid log format '%s', falling back to 'text'\n", format)
		format = "text"
	}

	lvl := slog.LevelInfo

	switch {
	case format == "text" || os.Getenv("LOG_LEVEL") == "debug":
		lvl = slog.LevelDebug
	}

	loggerOptions := slog.HandlerOptions{
		Level: lvl,
	}

	switch format {
	case "text":
		logHandler := LocalDevHandlerOptions{
			SlogOpts: loggerOptions,
			UseColor: true,
		}.NewLocalDevHandler(os.Stdout)
		logger = slog.New(logHandler)
	case "json":
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &loggerOptions))
	case "gcp":
		creds, err := google.FindDefaultCredentials(context.Background())
		if err != nil || creds.ProjectID == "" {
			fmt.Printf("failed to find default credentials (%v) or projectId empty: the traceId in logs will not be usable\n", err)
		}
		logger = slog.New(NewGcpHandler(creds.ProjectID, loggerOptions))
	}
	return logger
}

func StoreLoggerInContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ContextKeyLogger, logger)
}

func StoreLoggerInContextMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctxWithLogger := StoreLoggerInContext(c.Request.Context(), logger)
		c.Request = c.Request.WithContext(ctxWithLogger)
		c.Next()
	}
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	logger, found := ctx.Value(ContextKeyLogger).(*slog.Logger)
	if !found {
		logger = NewLogger("")
		logger.WarnContext(ctx, "logger not found in context. Falling back to a new logger, but it will be missing context keys")
	}
	return logger
}
