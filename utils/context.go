package utils

import (
	"context"
	"fmt"
	"log/slog"
	"marble/marble-backend/models"
	"net/http"
	"os"
	"regexp"

	"github.com/gofrs/uuid"
)

type ContextKey int

const (
	ContextKeyCredentials ContextKey = iota
	ContextKeyLogger
)

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

func AddStackdriverKeysToLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := LoggerFromContext(ctx)
		envDev := os.Getenv("ENV") != "DEV"
		projectId := os.Getenv("GCLOUD_PROJECT")

		var populateTraceId = func() string {
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

		traceId := populateTraceId()
		if projectId != "" {
			if traceId != "" {
				logger = logger.With("trace", fmt.Sprintf("projects/%s/traces/%s", projectId, traceId))
			} else if !envDev {
				logger.DebugContext(ctx, "no trace id found in request")
			}

			logger = logger.With("logName", fmt.Sprintf("projects/%s/logs/%s", projectId, "marble-backend"))
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, ContextKeyLogger, logger)))
	})
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
		panic(fmt.Errorf("logger not found context"))
	}
	return logger
}

func LogRequestError(r *http.Request, msg string, args ...any) {
	ctx := r.Context()
	LoggerFromContext(ctx).ErrorContext(ctx, msg, args...)
}

func CredentialsFromCtx(ctx context.Context) models.Credentials {

	creds, _ := ctx.Value(ContextKeyCredentials).(models.Credentials)
	return creds
}

func OrganizationIdFromRequest(request *http.Request) (organizationId string, err error) {

	creds := CredentialsFromCtx(request.Context())

	var requestOrganizationId string
	if request != nil {
		requestOrganizationId = request.URL.Query().Get("organization-id")
		if requestOrganizationId != "" {
			if err := ValidateUuid(requestOrganizationId); err != nil {
				return "", err
			}
		}
	}

	// allow organizationId to be passed in query param
	if requestOrganizationId != "" {
		if err := EnforceOrganizationAccess(creds, requestOrganizationId); err != nil {
			return "", err
		}
		return requestOrganizationId, nil
	}

	if creds.OrganizationId == "" {
		noMarbleAdmin := ""
		if creds.Role == models.MARBLE_ADMIN {
			noMarbleAdmin = "this Api is not supposed to be called with marble admin creds "
		}
		return "", fmt.Errorf("no organizationId in context. %s: %w", noMarbleAdmin, models.ForbiddenError)
	}

	return creds.OrganizationId, nil
}

// TODO: replace me with OrganizationIdFromContext
func OrgIDFromCtx(ctx context.Context, request *http.Request) (organizationId string, err error) {
	return OrganizationIdFromRequest(request)
}

func ValidateUuid(uuidParam string) error {
	_, err := uuid.FromString(uuidParam)
	if err != nil {
		err = fmt.Errorf("'%s' is not a valid UUID: %w", uuidParam, models.BadParameterError)
	}
	return err
}
