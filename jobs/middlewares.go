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
	"github.com/google/uuid"
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

// DemoOrgsFetcher is a function that returns the set of demo organization IDs
type DemoOrgsFetcher func(ctx context.Context) (map[uuid.UUID]struct{}, error)

type CronMonitorMiddleware struct {
	monitorConfigs  map[string]*sentry.MonitorConfig
	demoOrgsFetcher DemoOrgsFetcher
	demoOrgs        map[uuid.UUID]struct{}
	demoOrgsLock    sync.RWMutex
}

func (m *CronMonitorMiddleware) IsMiddleware() bool { return true }

func (m *CronMonitorMiddleware) isDemoOrg(orgId uuid.UUID) bool {
	m.demoOrgsLock.RLock()
	defer m.demoOrgsLock.RUnlock()
	_, isDemo := m.demoOrgs[orgId]
	return isDemo
}

func (m *CronMonitorMiddleware) refreshDemoOrgs(ctx context.Context) {
	if m.demoOrgsFetcher == nil {
		return
	}
	demoOrgs, err := m.demoOrgsFetcher(ctx)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return
	}
	m.demoOrgsLock.Lock()
	m.demoOrgs = demoOrgs
	m.demoOrgsLock.Unlock()
}

// StartDemoOrgsRefresh starts a background goroutine that periodically refreshes the demo orgs list
func (m *CronMonitorMiddleware) StartDemoOrgsRefresh(ctx context.Context, interval time.Duration) {
	// Initial fetch
	m.refreshDemoOrgs(ctx)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.refreshDemoOrgs(ctx)
			}
		}
	}()
}

func (m *CronMonitorMiddleware) Work(ctx context.Context, job *rivertype.JobRow, doInner func(context.Context) error) error {
	monitorConfig := m.monitorConfigs[job.Kind]
	if monitorConfig == nil {
		return doInner(ctx)
	}

	orgId, err := uuid.Parse(job.Queue)
	if err != nil {
		logger := utils.LoggerFromContext(ctx)
		logger.WarnContext(ctx, "CronMonitorMiddleware: unable to parse org ID from job queue. Processing to execute the task anyway.", "queue", job.Queue)
		return doInner(ctx)
	}

	// Skip monitoring for demo orgs
	if m.isDemoOrg(orgId) {
		return doInner(ctx)
	}

	slug := fmt.Sprintf("%s-%s", job.Kind, orgId.String())
	checkinId := sentry.CaptureCheckIn(&sentry.CheckIn{
		MonitorSlug: slug,
		Status:      sentry.CheckInStatusInProgress,
	}, monitorConfig)

	err = doInner(ctx)

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

func NewCronMonitorMiddleware(demoOrgsFetcher DemoOrgsFetcher) *CronMonitorMiddleware {
	return &CronMonitorMiddleware{
		monitorConfigs: map[string]*sentry.MonitorConfig{
			"scheduled_scenario": {
				Schedule:      sentry.IntervalSchedule(10, sentry.MonitorScheduleUnitMinute),
				CheckInMargin: 20, // allow 10 min late
				MaxRuntime:    5,  // 5 min max runtime
			},
			"webhook_retry": {
				Schedule:      sentry.IntervalSchedule(10, sentry.MonitorScheduleUnitMinute),
				CheckInMargin: 20,
				MaxRuntime:    5,
			},
		},
		demoOrgsFetcher: demoOrgsFetcher,
		demoOrgs:        make(map[uuid.UUID]struct{}),
	}
}
