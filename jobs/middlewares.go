package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	sentryErrorGroupingTime = 30 * time.Second
	sdkIdentifier           = "sentry.go.river.marble"
)

// Logger middleware

type LoggerMiddleware struct {
	l              *slog.Logger
	errorCount     map[string]int
	errorCountLock *sync.Mutex
}

func (m LoggerMiddleware) IsMiddleware() bool { return true }

func (m LoggerMiddleware) Work(ctx context.Context, job *rivertype.JobRow, doInner func(context.Context) error) error {
	logger := m.l.With(
		"job_id", job.ID,
		"job_kind", job.Kind,
		"job_attempt", job.Attempt,
		"last_attempted_at", job.AttemptedAt,
		"created_at", job.CreatedAt,
		"queue", job.Queue,
		"priority", job.Priority,
	)
	start := time.Now()
	logger.InfoContext(ctx, fmt.Sprintf("Starting %s job n°%d - attempt %d", job.Kind, job.ID, job.Attempt))

	ctx = utils.StoreLoggerInContext(ctx, logger)
	err := doInner(ctx)
	var snoozeErr *river.JobSnoozeError
	if err != nil && errors.As(err, &snoozeErr) {
		logger.InfoContext(ctx, fmt.Sprintf("%s job n°%d snoozed after %s", job.Kind, job.ID, time.Since(start)))
		return err
	} else if err != nil {
		logger.ErrorContext(ctx, fmt.Sprintf("%s job n°%d failed after %s", job.Kind, job.ID, time.Since(start)))
		m.aggregateAndLogError(ctx, job, err)
		return err
	}

	utils.MetricJobDuration.
		With(prometheus.Labels{"queue": job.Queue, "job_name": job.Kind}).
		Observe(time.Since(start).Seconds())

	logger.InfoContext(ctx, fmt.Sprintf("%s job n°%d succeeded after %s", job.Kind, job.ID, time.Since(start)))
	return nil
}

func (m LoggerMiddleware) aggregateAndLogError(ctx context.Context, job *rivertype.JobRow, err error) {
	m.errorCountLock.Lock()
	defer m.errorCountLock.Unlock()

	errorKey := fmt.Sprintf("%s:%s", job.Kind, err.Error())
	m.errorCount[errorKey]++

	if m.errorCount[errorKey] == 1 {
		go func() {
			time.Sleep(sentryErrorGroupingTime)
			m.errorCountLock.Lock()
			defer m.errorCountLock.Unlock()

			delete(m.errorCount, errorKey)

			utils.LogAndReportSentryError(ctx, err)
		}()
	}
}

func NewLoggerMiddleware(l *slog.Logger) LoggerMiddleware {
	return LoggerMiddleware{l: l, errorCount: make(map[string]int), errorCountLock: &sync.Mutex{}}
}

// Recovered middleware

type RecovererMiddleware struct{}

func (m RecovererMiddleware) IsMiddleware() bool { return true }

func (m RecovererMiddleware) Work(ctx context.Context, job *rivertype.JobRow, doInner func(context.Context) error) (err error) {
	defer utils.RecoverAndReportSentryError(ctx, "RecovererMiddleware.Work")
	return doInner(ctx)
}

func NewRecoveredMiddleware() RecovererMiddleware {
	return RecovererMiddleware{}
}

// Opentelemetry tracing middleware

type TracingMiddleware struct {
	tracer trace.Tracer
}

func (m TracingMiddleware) IsMiddleware() bool { return true }

func (m TracingMiddleware) Work(ctx context.Context, job *rivertype.JobRow, doInner func(context.Context) error) error {
	ctx, span := m.tracer.Start(
		ctx,
		job.Kind,
		trace.WithAttributes(
			attribute.Int64("job_id", job.ID),
			attribute.String("job_kind", job.Kind),
			attribute.Int("job_attempt", job.Attempt),
			attribute.String("last_attempted_at", job.AttemptedAt.Format(time.RFC3339)),
			attribute.String("created_at", job.CreatedAt.Format(time.RFC3339)),
			attribute.String("queue", job.Queue),
			attribute.Int("priority", job.Priority),
		),
	)
	defer span.End()

	return doInner(ctx)
}

func NewTracingMiddleware(tracer trace.Tracer) TracingMiddleware {
	return TracingMiddleware{tracer: tracer}
}

// Sentry middleware

type SentryMiddleware struct{}

func (m SentryMiddleware) IsMiddleware() bool { return true }

func (m SentryMiddleware) Work(ctx context.Context, job *rivertype.JobRow, doInner func(context.Context) error) error {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
		ctx = sentry.SetHubOnContext(ctx, hub)
	}

	if client := hub.Client(); client != nil {
		client.SetSDKIdentifier(sdkIdentifier)
	}

	options := []sentry.SpanOption{
		sentry.WithOpName("river.task"),
		sentry.WithTransactionSource(sentry.SourceTask),
	}

	scope := hub.PushScope()
	scope.SetTag("job_id", strconv.FormatInt(job.ID, 10))
	scope.SetTag("job_kind", job.Kind)
	scope.SetTag("job_attempt", strconv.Itoa(job.Attempt))
	scope.SetTag("queue", job.Queue)
	scope.SetTag("priority", strconv.Itoa(job.Priority))
	var args map[string]any
	if err := json.Unmarshal(job.EncodedArgs, &args); err != nil {
		scope.SetTag("payload", "error decoding payload")
	} else {
		scope.SetExtra("payload", args)
	}

	transaction := sentry.StartTransaction(ctx,
		fmt.Sprintf("river task %s", job.Kind),
		options...,
	)

	return doInner(transaction.Context())
}

func NewSentryMiddleware() SentryMiddleware {
	return SentryMiddleware{}
}

// Sentry Cron monitoring middleware
// This middleware reports job executions to Sentry Crons for monitoring scheduled tasks.
// It creates per-org monitors with slugs like `{job-kind}-{org-id}` and skips demo orgs.

type CronMonitorMiddleware struct {
	monitoredJobs map[string]time.Duration // job kind -> interval
}

func (m CronMonitorMiddleware) IsMiddleware() bool { return true }

func (m CronMonitorMiddleware) Work(ctx context.Context, job *rivertype.JobRow, doInner func(context.Context) error) error {
	interval, monitored := m.monitoredJobs[job.Kind]
	if !monitored {
		return doInner(ctx)
	}

	// Parse job args to check demo mode and get org ID
	var args struct {
		OrgId    string `json:"org_id"`
		DemoMode bool   `json:"demo_mode"`
	}
	if err := json.Unmarshal(job.EncodedArgs, &args); err != nil || args.OrgId == "" {
		return doInner(ctx)
	}

	// Skip monitoring for demo orgs
	if args.DemoMode {
		return doInner(ctx)
	}

	slug := fmt.Sprintf("%s-%s", job.Kind, args.OrgId)

	monitorConfig := &sentry.MonitorConfig{
		Schedule:      sentry.IntervalSchedule(int64(interval.Minutes()), sentry.MonitorScheduleUnitMinute),
		CheckInMargin: 2,  // allow 2 min late
		MaxRuntime:    60, // 1 hour max runtime
	}

	checkinId := sentry.CaptureCheckIn(&sentry.CheckIn{
		MonitorSlug: slug,
		Status:      sentry.CheckInStatusInProgress,
	}, monitorConfig)

	err := doInner(ctx)

	status := sentry.CheckInStatusOK
	if err != nil {
		status = sentry.CheckInStatusError
	}
	if checkinId != nil {
		sentry.CaptureCheckIn(&sentry.CheckIn{
			ID:          *checkinId,
			MonitorSlug: slug,
			Status:      status,
		}, nil)
	}
	return err
}

func NewCronMonitorMiddleware() CronMonitorMiddleware {
	return CronMonitorMiddleware{
		monitoredJobs: map[string]time.Duration{
			"scheduled_scenario": 10 * time.Minute,
			"webhook_retry":      10 * time.Minute,
		},
	}
}
