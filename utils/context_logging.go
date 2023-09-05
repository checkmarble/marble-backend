package utils

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
)

func NewLogger(env string) *slog.Logger {
	var logger *slog.Logger

	isDevEnv := env == "DEV"
	if isDevEnv {
		logHandler := LocalDevHandlerOptions{
			SlogOpts: slog.HandlerOptions{Level: slog.LevelDebug},
			UseColor: true,
		}.NewLocalDevHandler(os.Stdout)
		logger = slog.New(logHandler)
	} else {
		slogOption := slog.HandlerOptions{ReplaceAttr: GCPLoggerAttributeReplacer}
		jsonHandler := slog.NewJSONHandler(os.Stdout, &slogOption)
		logger = slog.New(jsonHandler)
	}
	return logger
}

func StoreLoggerInContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ContextKeyLogger, logger)
}

func StoreLoggerInContextMiddleware(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctxWithLogger := StoreLoggerInContext(r.Context(), logger)
			next.ServeHTTP(w, r.WithContext(ctxWithLogger))
		})
	}
}

func AddTraceIdToLoggerMiddleware(isDevEnv bool, projectId string) func(next http.Handler) http.Handler {
	// Returns a middleware that adds the trace key to the logger, if the projectId is found
	// and the trace header is present
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			logger := LoggerFromContext(ctx)

			var findTraceId = func() string {
				header := r.Header.Get("Traceparent")
				if header != "" {
					traceId, _ := deconstructTraceParent(header)
					if traceId != "" {
						return traceId
					}
				}

				header = r.Header.Get("X-Cloud-Trace-Context")
				if header != "" {
					traceId, _, _ := deconstructXCloudTraceContext(header)
					return traceId

				}
				return ""
			}

			traceId := findTraceId()
			if projectId != "" {
				if traceId != "" {
					logger = logger.With("logging.googleapis.com/trace", fmt.Sprintf("projects/%s/traces/%s", projectId, traceId))
				} else if !isDevEnv {
					logger.DebugContext(ctx, "no trace id found in request")
				}
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, ContextKeyLogger, logger)))
		})
	}
}

// As per format described at https://www.w3.org/TR/trace-context/#traceparent-header-field-values
var validTraceParentExpression = regexp.MustCompile(`^(00)-([a-fA-F\d]{32})-([a-f\d]{16})-([a-fA-F\d]{2})$`)

func deconstructTraceParent(s string) (traceID, spanID string) {
	matches := validTraceParentExpression.FindStringSubmatch(s)
	if matches != nil {
		// regexp package does not support negative lookahead preventing all 0 validations
		if matches[2] == "00000000000000000000000000000000" || matches[3] == "0000000000000000" {
			return
		}
		traceID, spanID = matches[2], matches[3]
	}
	return
}

var validXCloudTraceContext = regexp.MustCompile(
	// Matches on "TRACE_ID"
	`([a-f\d]+)?` +
		// Matches on "/SPAN_ID"
		`(?:/([a-f\d]+))?` +
		// Matches on ";0=TRACE_TRUE"
		`(?:;o=(\d))?`)

func deconstructXCloudTraceContext(s string) (traceID, spanID string, traceSampled bool) {
	// As per the format described at https://cloud.google.com/trace/docs/setup#force-trace
	//    "X-Cloud-Trace-Context: TRACE_ID/SPAN_ID;o=TRACE_TRUE"
	// for example:
	//    "X-Cloud-Trace-Context: 105445aa7843bc8bf206b120001000/1;o=1"
	//
	// We expect:
	//   * traceID (optional): 			"105445aa7843bc8bf206b120001000"
	//   * spanID (optional):       	"1"
	//   * traceSampled (optional): 	true
	matches := validXCloudTraceContext.FindStringSubmatch(s)

	if matches != nil {
		traceID, spanID, traceSampled = matches[1], matches[2], matches[3] == "1"
	}

	if spanID == "0" {
		spanID = ""
	}

	return
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	logger, found := ctx.Value(ContextKeyLogger).(*slog.Logger)
	if !found {
		logger := NewLogger("")
		logger.ErrorContext(ctx, "logger not found in context. Falling back to a new logger, but it will be missing context keys")
	}
	return logger
}

func LogRequestError(r *http.Request, msg string, args ...any) {
	ctx := r.Context()
	LoggerFromContext(ctx).ErrorContext(ctx, msg, args...)
}
