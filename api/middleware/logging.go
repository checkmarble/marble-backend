package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type config struct {
	logger     *slog.Logger
	ignorePath []string

	defaultLevel     slog.Level
	clientErrorLevel slog.Level
	serverErrorLevel slog.Level
}

type LoggerOption func(*config)

func WithIgnorePath(s []string) LoggerOption {
	return func(c *config) {
		c.ignorePath = s
	}
}

func NewLogging(logger *slog.Logger, options ...LoggerOption) gin.HandlerFunc {
	l := &config{
		logger:           logger,
		defaultLevel:     slog.LevelInfo,
		clientErrorLevel: slog.LevelWarn,
		serverErrorLevel: slog.LevelError,
	}

	for _, option := range options {
		option(l)
	}

	ignore := make(map[string]struct{}, len(l.ignorePath))
	for _, path := range l.ignorePath {
		ignore[path] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, ok := ignore[c.Request.URL.Path]; ok {
			return
		}

		path := c.Request.URL.Path
		start := time.Now()
		c.Next()
		stop := time.Since(start)
		latency := stop.Milliseconds()
		status := c.Writer.Status()
		IP := c.ClientIP()
		userAgent := c.Request.UserAgent()
		dataLength := c.Writer.Size()
		if dataLength < 0 {
			dataLength = 0
		}

		level := l.defaultLevel
		if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
			level = l.clientErrorLevel
		}
		if status >= http.StatusInternalServerError {
			level = l.serverErrorLevel
		}

		attributes := []slog.Attr{
			slog.Int("status", status),
			slog.Int64("latency", latency),
			slog.String("client_ip", IP),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("data_length", dataLength),
			slog.String("user_agent", userAgent),
		}
		if c.Errors != nil {
			attributes = append(attributes, slog.String("error", c.Errors.String()))
		}
		l.logger.LogAttrs(c.Request.Context(), level,
			fmt.Sprintf("%s %s", c.Request.Method, path), attributes...)
	}
}
