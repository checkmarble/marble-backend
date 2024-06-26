package utils

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"

	"github.com/gin-gonic/gin"
)

func NewLogger(format string) *slog.Logger {
	var logger *slog.Logger
	if !slices.Contains([]string{"text", "json", "gcp"}, format) {
		fmt.Printf("invalid log format '%s', falling back to 'text'\n", format)
		format = "text"
	}

	switch format {
	case "text":
		logHandler := LocalDevHandlerOptions{
			SlogOpts: slog.HandlerOptions{Level: slog.LevelDebug},
			UseColor: true,
		}.NewLocalDevHandler(os.Stdout)
		logger = slog.New(logHandler)
	case "json":
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	case "gcp":
		projectId := GetEnv("GOOGLE_CLOUD_PROJECT", "")
		if projectId == "" {
			fmt.Println("GOOGLE_CLOUD_PROJECT not set, the trace id in logs will not be usable")
		}
		logger = slog.New(NewGcpHandler(projectId))
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
		logger.ErrorContext(ctx, "logger not found in context. Falling back to a new logger, but it will be missing context keys")
	}
	return logger
}

func LogRequestError(r *http.Request, msg string, args ...any) {
	ctx := r.Context()
	LoggerFromContext(ctx).ErrorContext(ctx, msg, args...)
}

func LogRequestInfo(r *http.Request, msg string, args ...any) {
	ctx := r.Context()
	LoggerFromContext(ctx).InfoContext(ctx, msg, args...)
}
