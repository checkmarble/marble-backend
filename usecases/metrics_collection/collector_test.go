package metrics_collection

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

// Mock implementations for testing
type MockCollectorRepository struct {
	mock.Mock
}

func (m *MockCollectorRepository) AllOrganizations(ctx context.Context,
	exec repositories.Executor,
) ([]models.Organization, error) {
	args := m.Called(ctx, exec)
	return args.Get(0).([]models.Organization), args.Error(1)
}

func (m *MockCollectorRepository) CountDecisionsByOrg(ctx context.Context, exec repositories.Executor, orgIds []string,
	from, to time.Time,
) (map[string]int, error) {
	args := m.Called(ctx, exec, orgIds, from, to)
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockCollectorRepository) CountCasesByOrg(ctx context.Context, exec repositories.Executor, orgIds []string,
	from, to time.Time,
) (map[string]int, error) {
	args := m.Called(ctx, exec, orgIds, from, to)
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockCollectorRepository) CountScreeningsByOrg(ctx context.Context, exec repositories.Executor, orgIds []string,
	from, to time.Time,
) (map[string]int, error) {
	args := m.Called(ctx, exec, orgIds, from, to)
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockCollectorRepository) CountScreeningsByProvider(ctx context.Context, exec repositories.Executor, orgIds []string, providers []models.ScreeningProvider,
	from, to time.Time,
) (models.ByOrgByProviderCounter, error) {
	args := m.Called(ctx, exec, orgIds, providers, from, to)
	return args.Get(0).(models.ByOrgByProviderCounter), args.Error(1)
}

func (m *MockCollectorRepository) CountAiCaseReviewsByOrg(ctx context.Context, exec repositories.Executor, orgIds []string,
	from, to time.Time,
) (map[string]int, error) {
	args := m.Called(ctx, exec, orgIds, from, to)
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockCollectorRepository) GetMetadata(ctx context.Context, exec repositories.Executor, orgID *uuid.UUID,
	key models.MetadataKey,
) (*models.Metadata, error) {
	args := m.Called(ctx, exec, orgID, key)
	return args.Get(0).(*models.Metadata), args.Error(1)
}

func (m *MockCollectorRepository) GetEnabledConfigStableIdsByOrg(ctx context.Context, exec repositories.Executor,
	orgIds []string,
) (map[string][]uuid.UUID, error) {
	args := m.Called(ctx, exec, orgIds)
	return args.Get(0).(map[string][]uuid.UUID), args.Error(1)
}

func (m *MockCollectorRepository) CountCSScreeningsByProvider(ctx context.Context, exec repositories.Executor,
	orgIds []string, providers []models.ScreeningProvider, from, to time.Time,
) (models.ByOrgByProviderCounter, error) {
	args := m.Called(ctx, exec, orgIds, providers, from, to)
	return args.Get(0).(models.ByOrgByProviderCounter), args.Error(1)
}

func (m *MockCollectorRepository) CountFreeformSearchesByProvider(ctx context.Context, exec repositories.Executor,
	orgIds []string, providers []models.ScreeningProvider, from, to time.Time,
) (models.ByOrgByProviderCounter, error) {
	args := m.Called(ctx, exec, orgIds, providers, from, to)
	return args.Get(0).(models.ByOrgByProviderCounter), args.Error(1)
}

func (m *MockCollectorRepository) GetPostgresVersion(ctx context.Context, exec repositories.Executor) (int, error) {
	args := m.Called(ctx, exec)
	return args.Get(0).(int), args.Error(1)
}

type MockCollectorClientRepository struct {
	mock.Mock
}

func (m *MockCollectorClientRepository) CountMonitoredObjectsByConfigStableIds(ctx context.Context, exec repositories.Executor,
	configStableIds []uuid.UUID,
) (int, error) {
	args := m.Called(ctx, exec, configStableIds)
	return args.Get(0).(int), args.Error(1)
}

func (m *MockCollectorClientRepository) IsContinuousScreeningSetup(ctx context.Context, exec repositories.Executor) (bool, error) {
	args := m.Called(ctx, exec)
	return args.Bool(0), args.Error(1)
}

func (m *MockCollectorClientRepository) CountActiveMonitoredObjects(ctx context.Context, exec repositories.Executor,
	yearStart, yearEnd time.Time,
) (int, error) {
	args := m.Called(ctx, exec, yearStart, yearEnd)
	return args.Get(0).(int), args.Error(1)
}

type MockGlobalCollector struct {
	mock.Mock
}

func (m *MockGlobalCollector) Collect(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]models.MetricData), args.Error(1)
}

type MockCollector struct {
	mock.Mock
}

func (m *MockCollector) Collect(ctx context.Context, orgs []models.Organization, from time.Time, to time.Time) ([]models.MetricData, error) {
	args := m.Called(ctx, orgs, from, to)
	return args.Get(0).([]models.MetricData), args.Error(1)
}

func TestNewCollectorsV1(t *testing.T) {
	// Clear cache after test to ensure clean state for subsequent tests
	t.Cleanup(DeploymentIDCache.Purge)

	// Setup
	mockRepository := new(MockCollectorRepository)
	mockClientRepository := new(MockCollectorClientRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()

	// Execute
	collectors := NewCollectorsV1(
		mockExecutorFactory,
		mockRepository,
		mockClientRepository,
		"ApiVersionTest",
		models.LicenseConfiguration{},
	)

	// Assert
	assert.Equal(t, "v1", collectors.version)
	assert.Len(t, collectors.globalCollectors, 2)
	assert.Len(t, collectors.counterCollectors, 7)
	assert.Len(t, collectors.snapshotCollectors, 1)
	assert.Equal(t, mockRepository, collectors.repository)
	assert.Equal(t, mockExecutorFactory, collectors.executorFactory)

	// Verify the collectors are of the expected stub types
	_, isAppVersionCollector := collectors.globalCollectors[0].(AppVersionCollector)
	assert.True(t, isAppVersionCollector, "Should contain AppVersionCollector")
	_, isPostgresVersionCollector := collectors.globalCollectors[1].(PostgresVersionCollector)
	assert.True(t, isPostgresVersionCollector, "Should contain PostgresVersionCollector")

	_, isDecisionCollector := collectors.counterCollectors[0].(DecisionCollector)
	assert.True(t, isDecisionCollector, "Should contain DecisionCollector")

	_, isCaseCollector := collectors.counterCollectors[1].(CaseCollector)
	assert.True(t, isCaseCollector, "Should contain CaseCollector")

	_, isScreeningCollector := collectors.counterCollectors[2].(ScreeningCollector)
	assert.True(t, isScreeningCollector, "Should contain ScreeningCollector")

	_, isAiCaseReviewCollector := collectors.counterCollectors[3].(AiCaseReviewCollector)
	assert.True(t, isAiCaseReviewCollector, "Should contain AiCaseReviewCollector")

	_, isMonitoredObjectCollector := collectors.snapshotCollectors[0].(MonitoredObjectCollector)
	assert.True(t, isMonitoredObjectCollector, "Should contain MonitoredObjectCollector")

	_, isScreeningByProviderCollector := collectors.counterCollectors[4].(ScreeningByProviderCollector)
	assert.True(t, isScreeningByProviderCollector, "Should contain ScreeningByProviderCollector")

	_, isContinuousScreeningByProviderCollector := collectors.counterCollectors[5].(ContinuousScreeningByProviderCollector)
	assert.True(t, isContinuousScreeningByProviderCollector, "Should contain ContinuousScreeningByProviderCollector")

	_, isFreeformSearchByProviderCollector := collectors.counterCollectors[6].(FreeformSearchByProviderCollector)
	assert.True(t, isFreeformSearchByProviderCollector, "Should contain FreeformSearchByProviderCollector")
}

func TestCounterCollectorNames(t *testing.T) {
	t.Cleanup(DeploymentIDCache.Purge)

	collectors := NewCollectorsV1(
		executor_factory.NewExecutorFactoryStub(),
		new(MockCollectorRepository),
		new(MockCollectorClientRepository),
		"ApiVersionTest",
		models.LicenseConfiguration{},
	)

	// Hardcoded on purpose: renaming a constant must break this test, since names are persisted as watermark specs.
	expected := []string{
		"decisions",
		"cases",
		"screenings",
		"ai_case_reviews",
		"screenings_by_provider",
		"continuous_screenings_by_provider",
		"freeform_searches_by_provider",
	}

	seen := make(map[string]struct{}, len(collectors.counterCollectors))
	for _, cc := range collectors.counterCollectors {
		name := cc.Name()
		assert.NotEmpty(t, name)
		assert.NotContains(t, name, ":", "counter collector name %q must not contain ':'", name)
		_, duplicate := seen[name]
		assert.False(t, duplicate, "duplicate counter collector name %q", name)
		seen[name] = struct{}{}
	}

	assert.Equal(t, expected, collectors.CounterCollectorNames())
}

// MockCounterCollector is a CounterCollector with a fixed name, used to test the counter orchestration.
type MockCounterCollector struct {
	mock.Mock
	name string
}

func (m *MockCounterCollector) Name() string {
	return m.name
}

func (m *MockCounterCollector) Collect(ctx context.Context, orgs []models.Organization, from time.Time, to time.Time) ([]models.MetricData, error) {
	args := m.Called(ctx, orgs, from, to)
	return args.Get(0).([]models.MetricData), args.Error(1)
}

// Shared fixtures for the CollectMetrics tests. The clock is fixed so that daily periods are deterministic.
var (
	metricsTestNow          = time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC)
	metricsTestDeploymentID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	metricsTestOrgs         = []models.Organization{{Id: uuid.MustParse("22222222-2222-2222-2222-222222222222")}}
	metricsTestValue        = []models.MetricData{models.NewGlobalMetric("test_metric", nil, nil, time.Time{}, time.Time{})}
	metricsTestNoValue      []models.MetricData
	metricsTestBoom         = errors.New("boom")
)

// metricsTestAt returns 2024-01-<day> at <hour>:00 UTC.
func metricsTestAt(day, hour int) time.Time {
	return time.Date(2024, 1, day, hour, 0, 0, 0, time.UTC)
}

func newMetricsTestCollectors(t *testing.T, repo *MockCollectorRepository,
	globals []GlobalCollector, snapshots []Collector, counters []CounterCollector,
) Collectors {
	t.Helper()
	DeploymentIDCache.Purge()
	t.Cleanup(DeploymentIDCache.Purge)

	return Collectors{
		version:            "v1",
		globalCollectors:   globals,
		snapshotCollectors: snapshots,
		counterCollectors:  counters,
		repository:         repo,
		executorFactory:    executor_factory.NewExecutorFactoryStub(),
	}
}

// expectDeploymentIDAndOrgs makes the repository answer the two lookups every run starts with.
func expectDeploymentIDAndOrgs(repo *MockCollectorRepository) {
	repo.On("GetMetadata", mock.Anything, mock.Anything, mock.Anything, models.MetadataKeyDeploymentID).
		Return(&models.Metadata{Value: metricsTestDeploymentID.String()}, nil)
	repo.On("AllOrganizations", mock.Anything, mock.Anything).Return(metricsTestOrgs, nil)
}

func expectCounterPeriod(c *MockCounterCollector, from, to time.Time) *mock.Call {
	return c.On("Collect", mock.Anything, mock.Anything, from, to).Return(metricsTestValue, nil)
}

// blockUntilDone simulates a collector that hangs until its context is done.
func blockUntilDone(args mock.Arguments) {
	<-args.Get(0).(context.Context).Done()
}

func recordCollectorCall(order *[]string, name string) func(mock.Arguments) {
	return func(mock.Arguments) {
		*order = append(*order, name)
	}
}

func TestCollectors_CollectMetrics_CaughtUp(t *testing.T) {
	repo := new(MockCollectorRepository)
	expectDeploymentIDAndOrgs(repo)
	counter := &MockCounterCollector{name: "decisions"}

	from := metricsTestAt(8, 15)
	calls := []*mock.Call{
		expectCounterPeriod(counter, metricsTestAt(8, 15), metricsTestAt(9, 0)),
		expectCounterPeriod(counter, metricsTestAt(9, 0), metricsTestAt(10, 0)),
		expectCounterPeriod(counter, metricsTestAt(10, 0), metricsTestNow),
	}
	mock.InOrder(calls...)
	collectors := newMetricsTestCollectors(t, repo, nil, nil, []CounterCollector{counter})

	res, err := collectors.CollectMetrics(context.Background(), models.MetricsCollectionRequest{
		Now:              metricsTestNow,
		SnapshotFrom:     from,
		CounterFrom:      map[string]time.Time{"decisions": from},
		CollectorTimeout: time.Minute,
	})

	require.NoError(t, err)
	require.Len(t, res.Progress, 1)
	assert.Equal(t, models.MetricsCounterProgress{
		CollectorName:  "decisions",
		From:           from,
		CompletedUntil: metricsTestNow,
	}, res.Progress[0])
	assert.Len(t, res.Collection.Metrics, 3)
	assert.Equal(t, metricsTestDeploymentID, res.Collection.DeploymentID)
	counter.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestCollectors_CollectMetrics_CounterTimeoutMidBacklog(t *testing.T) {
	repo := new(MockCollectorRepository)
	expectDeploymentIDAndOrgs(repo)
	counter := &MockCounterCollector{name: "decisions"}

	from := metricsTestAt(8, 15)
	p1To := metricsTestAt(9, 0)
	p2To := metricsTestAt(10, 0)
	expectCounterPeriod(counter, from, p1To)
	expectCounterPeriod(counter, p1To, p2To)
	counter.On("Collect", mock.Anything, mock.Anything, p2To, metricsTestNow).
		Run(blockUntilDone).Return(metricsTestNoValue, context.DeadlineExceeded)
	collectors := newMetricsTestCollectors(t, repo, nil, nil, []CounterCollector{counter})

	res, err := collectors.CollectMetrics(context.Background(), models.MetricsCollectionRequest{
		Now:              metricsTestNow,
		SnapshotFrom:     from,
		CounterFrom:      map[string]time.Time{"decisions": from},
		CollectorTimeout: 200 * time.Millisecond,
	})

	require.NoError(t, err)
	require.Len(t, res.Progress, 1)
	progress := res.Progress[0]
	assert.Equal(t, p2To, progress.CompletedUntil)
	assert.ErrorIs(t, progress.Err, context.DeadlineExceeded)
	assert.Len(t, res.Collection.Metrics, 2)
	counter.AssertNumberOfCalls(t, "Collect", 3)
}

func TestCollectors_CollectMetrics_CounterErrorStopsOnlyThatCollector(t *testing.T) {
	repo := new(MockCollectorRepository)
	expectDeploymentIDAndOrgs(repo)
	failing := &MockCounterCollector{name: "a"}
	healthy := &MockCounterCollector{name: "b"}

	from := metricsTestAt(8, 15)
	p1To := metricsTestAt(9, 0)
	p2To := metricsTestAt(10, 0)
	expectCounterPeriod(failing, from, p1To)
	failing.On("Collect", mock.Anything, mock.Anything, p1To, p2To).Return(metricsTestNoValue, metricsTestBoom)
	expectCounterPeriod(healthy, from, p1To)
	expectCounterPeriod(healthy, p1To, p2To)
	expectCounterPeriod(healthy, p2To, metricsTestNow)
	collectors := newMetricsTestCollectors(t, repo, nil, nil, []CounterCollector{failing, healthy})

	res, err := collectors.CollectMetrics(context.Background(), models.MetricsCollectionRequest{
		Now:              metricsTestNow,
		SnapshotFrom:     from,
		CounterFrom:      map[string]time.Time{"a": from, "b": from},
		CollectorTimeout: time.Minute,
	})

	require.NoError(t, err)
	require.Len(t, res.Progress, 2)
	a, b := res.Progress[0], res.Progress[1]
	assert.Equal(t, "a", a.CollectorName)
	assert.Equal(t, p1To, a.CompletedUntil)
	assert.ErrorIs(t, a.Err, metricsTestBoom)
	assert.Equal(t, "b", b.CollectorName)
	assert.Equal(t, metricsTestNow, b.CompletedUntil)
	assert.NoError(t, b.Err)
	failing.AssertNumberOfCalls(t, "Collect", 2)
	healthy.AssertNumberOfCalls(t, "Collect", 3)
}

func TestCollectors_CollectMetrics_BudgetAlreadyExhausted(t *testing.T) {
	repo := new(MockCollectorRepository)
	expectDeploymentIDAndOrgs(repo)
	counter := &MockCounterCollector{name: "decisions"}
	collectors := newMetricsTestCollectors(t, repo, nil, nil, []CounterCollector{counter})

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	from := metricsTestAt(8, 15)

	res, err := collectors.CollectMetrics(ctx, models.MetricsCollectionRequest{
		Now:              metricsTestNow,
		SnapshotFrom:     from,
		CounterFrom:      map[string]time.Time{"decisions": from},
		CollectorTimeout: time.Minute,
	})

	require.NoError(t, err)
	require.Len(t, res.Progress, 1)
	progress := res.Progress[0]
	assert.Equal(t, from, progress.CompletedUntil)
	assert.ErrorIs(t, progress.Err, context.DeadlineExceeded)
	counter.AssertNotCalled(t, "Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCollectors_CollectMetrics_BudgetCapsCounters(t *testing.T) {
	repo := new(MockCollectorRepository)
	expectDeploymentIDAndOrgs(repo)
	first := &MockCounterCollector{name: "a"}
	second := &MockCounterCollector{name: "b"}
	first.On("Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(blockUntilDone).Return(metricsTestNoValue, context.DeadlineExceeded)
	collectors := newMetricsTestCollectors(t, repo, nil, nil, []CounterCollector{first, second})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	from := metricsTestAt(8, 15)

	start := time.Now()
	res, err := collectors.CollectMetrics(ctx, models.MetricsCollectionRequest{
		Now:              metricsTestNow,
		SnapshotFrom:     from,
		CounterFrom:      map[string]time.Time{"a": from, "b": from},
		CollectorTimeout: time.Hour,
	})
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, elapsed, time.Second)
	require.Len(t, res.Progress, 2)
	assert.ErrorIs(t, res.Progress[0].Err, context.DeadlineExceeded)
	assert.Equal(t, from, res.Progress[0].CompletedUntil)
	assert.ErrorContains(t, res.Progress[1].Err, "metrics collection budget exhausted")
	assert.Equal(t, from, res.Progress[1].CompletedUntil)
	second.AssertNotCalled(t, "Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCollectors_CollectMetrics_GaugesRunOnceNotSplit(t *testing.T) {
	repo := new(MockCollectorRepository)
	expectDeploymentIDAndOrgs(repo)
	snapshotFrom := metricsTestNow.AddDate(0, 0, -10)

	global := &MockGlobalCollector{}
	global.On("Collect", mock.Anything, snapshotFrom, metricsTestNow).Return(metricsTestValue, nil).Once()
	snapshot := &MockCollector{}
	snapshot.On("Collect", mock.Anything, mock.Anything, snapshotFrom, metricsTestNow).Return(metricsTestValue, nil).Once()
	collectors := newMetricsTestCollectors(t, repo, []GlobalCollector{global}, []Collector{snapshot}, nil)

	res, err := collectors.CollectMetrics(context.Background(), models.MetricsCollectionRequest{
		Now:              metricsTestNow,
		SnapshotFrom:     snapshotFrom,
		CollectorTimeout: time.Minute,
	})

	require.NoError(t, err)
	assert.Empty(t, res.Progress)
	assert.Len(t, res.Collection.Metrics, 2)
	global.AssertNumberOfCalls(t, "Collect", 1)
	snapshot.AssertNumberOfCalls(t, "Collect", 1)
}

func TestCollectors_CollectMetrics_GaugeFailureDoesNotStopCounters(t *testing.T) {
	tests := []struct {
		name           string
		globalErr      error
		snapshotBlocks bool
	}{
		{name: "global collector fails", globalErr: metricsTestBoom},
		{name: "snapshot collector blocks past its timeout", snapshotBlocks: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockCollectorRepository)
			expectDeploymentIDAndOrgs(repo)

			global := &MockGlobalCollector{}
			if tt.globalErr != nil {
				global.On("Collect", mock.Anything, mock.Anything, mock.Anything).Return(metricsTestNoValue, tt.globalErr)
			} else {
				global.On("Collect", mock.Anything, mock.Anything, mock.Anything).Return(metricsTestValue, nil)
			}

			snapshot := &MockCollector{}
			if tt.snapshotBlocks {
				snapshot.On("Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Run(blockUntilDone).Return(metricsTestNoValue, context.DeadlineExceeded)
			} else {
				snapshot.On("Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(metricsTestValue, nil)
			}

			counter := &MockCounterCollector{name: "decisions"}
			from := metricsTestAt(9, 0)
			expectCounterPeriod(counter, from, metricsTestAt(10, 0))
			expectCounterPeriod(counter, metricsTestAt(10, 0), metricsTestNow)
			collectors := newMetricsTestCollectors(t, repo,
				[]GlobalCollector{global}, []Collector{snapshot}, []CounterCollector{counter})

			res, err := collectors.CollectMetrics(context.Background(), models.MetricsCollectionRequest{
				Now:              metricsTestNow,
				SnapshotFrom:     from,
				CounterFrom:      map[string]time.Time{"decisions": from},
				CollectorTimeout: 200 * time.Millisecond,
			})

			require.NoError(t, err)
			require.Len(t, res.Progress, 1)
			assert.Equal(t, metricsTestNow, res.Progress[0].CompletedUntil)
			assert.NoError(t, res.Progress[0].Err)
			counter.AssertNumberOfCalls(t, "Collect", 2)
		})
	}
}

func TestCollectors_CollectMetrics_BacklogOfNDaysFullyProcessedInOneRun(t *testing.T) {
	const days = 10
	repo := new(MockCollectorRepository)
	expectDeploymentIDAndOrgs(repo)
	counter := &MockCounterCollector{name: "decisions"}

	// 10-day backlog up to now: one daily period per day, all processed without a per-run cap.
	from := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	now := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	for i := range days {
		expectCounterPeriod(counter, from.AddDate(0, 0, i), from.AddDate(0, 0, i+1))
	}
	collectors := newMetricsTestCollectors(t, repo, nil, nil, []CounterCollector{counter})

	res, err := collectors.CollectMetrics(context.Background(), models.MetricsCollectionRequest{
		Now:              now,
		SnapshotFrom:     from,
		CounterFrom:      map[string]time.Time{"decisions": from},
		CollectorTimeout: time.Minute,
	})

	require.NoError(t, err)
	require.Len(t, res.Progress, 1)
	progress := res.Progress[0]
	assert.Equal(t, now, progress.CompletedUntil)
	assert.NoError(t, progress.Err)
	counter.AssertNumberOfCalls(t, "Collect", days)
}

func TestCollectors_CollectMetrics_NoWorkWhenFromNotBeforeNow(t *testing.T) {
	tests := []struct {
		name string
		from time.Time
	}{
		{name: "from equals now", from: metricsTestNow},
		{name: "from after now", from: metricsTestNow.Add(time.Hour)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockCollectorRepository)
			expectDeploymentIDAndOrgs(repo)
			counter := &MockCounterCollector{name: "decisions"}
			collectors := newMetricsTestCollectors(t, repo, nil, nil, []CounterCollector{counter})

			res, err := collectors.CollectMetrics(context.Background(), models.MetricsCollectionRequest{
				Now:              metricsTestNow,
				SnapshotFrom:     tt.from,
				CounterFrom:      map[string]time.Time{"decisions": tt.from},
				CollectorTimeout: time.Minute,
			})

			require.NoError(t, err)
			require.Len(t, res.Progress, 1)
			assert.Equal(t, tt.from, res.Progress[0].CompletedUntil)
			assert.NoError(t, res.Progress[0].Err)
			counter.AssertNotCalled(t, "Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestCollectors_CollectMetrics_RunsOldestWatermarkFirst(t *testing.T) {
	repo := new(MockCollectorRepository)
	expectDeploymentIDAndOrgs(repo)
	var order []string

	// "a" is listed first and has the newest watermark, "b" has the oldest: "b" must run first.
	younger := &MockCounterCollector{name: "a"}
	older := &MockCounterCollector{name: "b"}
	younger.On("Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(recordCollectorCall(&order, "a")).Return(metricsTestValue, nil)
	older.On("Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(recordCollectorCall(&order, "b")).Return(metricsTestValue, nil)
	collectors := newMetricsTestCollectors(t, repo, nil, nil, []CounterCollector{younger, older})

	res, err := collectors.CollectMetrics(context.Background(), models.MetricsCollectionRequest{
		Now:              metricsTestNow,
		SnapshotFrom:     metricsTestAt(5, 0),
		CounterFrom:      map[string]time.Time{"a": metricsTestAt(8, 15), "b": metricsTestAt(5, 0)},
		CollectorTimeout: time.Minute,
	})

	require.NoError(t, err)
	// "b" backlog from Jan 5 00:00 is 6 daily periods, "a" from Jan 8 15:00 is 3.
	assert.Equal(t, []string{"b", "b", "b", "b", "b", "b", "a", "a", "a"}, order)
	require.Len(t, res.Progress, 2)
	assert.Equal(t, "b", res.Progress[0].CollectorName)
	assert.Equal(t, "a", res.Progress[1].CollectorName)
	assert.Equal(t, metricsTestNow, res.Progress[0].CompletedUntil)
	assert.Equal(t, metricsTestNow, res.Progress[1].CompletedUntil)
}

func TestCollectors_CollectMetrics_SkipsCounterMissingFromRequest(t *testing.T) {
	repo := new(MockCollectorRepository)
	expectDeploymentIDAndOrgs(repo)
	absent := &MockCounterCollector{name: "absent"}
	present := &MockCounterCollector{name: "present"}

	from := metricsTestAt(9, 0)
	expectCounterPeriod(present, from, metricsTestAt(10, 0))
	expectCounterPeriod(present, metricsTestAt(10, 0), metricsTestNow)
	collectors := newMetricsTestCollectors(t, repo, nil, nil, []CounterCollector{absent, present})

	res, err := collectors.CollectMetrics(context.Background(), models.MetricsCollectionRequest{
		Now:              metricsTestNow,
		SnapshotFrom:     from,
		CounterFrom:      map[string]time.Time{"present": from},
		CollectorTimeout: time.Minute,
	})

	require.NoError(t, err)
	require.Len(t, res.Progress, 1)
	assert.Equal(t, "present", res.Progress[0].CollectorName)
	absent.AssertNotCalled(t, "Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCollectors_CollectMetrics_SetupErrorsAbortRun(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(repo *MockCollectorRepository)
		expectErr error
	}{
		{
			name: "deployment ID lookup fails",
			setup: func(repo *MockCollectorRepository) {
				repo.On("GetMetadata", mock.Anything, mock.Anything, mock.Anything, models.MetadataKeyDeploymentID).
					Return((*models.Metadata)(nil), metricsTestBoom)
			},
			expectErr: metricsTestBoom,
		},
		{
			name: "deployment ID not found",
			setup: func(repo *MockCollectorRepository) {
				repo.On("GetMetadata", mock.Anything, mock.Anything, mock.Anything, models.MetadataKeyDeploymentID).
					Return((*models.Metadata)(nil), nil)
			},
			expectErr: models.NotFoundError,
		},
		{
			name: "organization listing fails",
			setup: func(repo *MockCollectorRepository) {
				repo.On("GetMetadata", mock.Anything, mock.Anything, mock.Anything, models.MetadataKeyDeploymentID).
					Return(&models.Metadata{Value: metricsTestDeploymentID.String()}, nil)
				repo.On("AllOrganizations", mock.Anything, mock.Anything).
					Return([]models.Organization(nil), metricsTestBoom)
			},
			expectErr: metricsTestBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockCollectorRepository)
			tt.setup(repo)
			global := &MockGlobalCollector{}
			counter := &MockCounterCollector{name: "decisions"}
			collectors := newMetricsTestCollectors(t, repo,
				[]GlobalCollector{global}, nil, []CounterCollector{counter})

			from := metricsTestAt(8, 15)
			_, err := collectors.CollectMetrics(context.Background(), models.MetricsCollectionRequest{
				Now:              metricsTestNow,
				SnapshotFrom:     from,
				CounterFrom:      map[string]time.Time{"decisions": from},
				CollectorTimeout: time.Minute,
			})

			assert.ErrorIs(t, err, tt.expectErr)
			global.AssertNotCalled(t, "Collect", mock.Anything, mock.Anything, mock.Anything)
			counter.AssertNotCalled(t, "Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestCollectors_CollectMetrics_CollectorTimeoutDoesNotStopLaterCollectors(t *testing.T) {
	repo := new(MockCollectorRepository)
	expectDeploymentIDAndOrgs(repo)

	// A starts first and hangs on its 2nd period until its own CollectorTimeout fires.
	// The parent ctx stays alive, so B (later From, runs second) must still complete.
	a := &MockCounterCollector{name: "a"}
	aFrom := metricsTestAt(8, 15)
	aP1To := metricsTestAt(9, 0)
	aP2To := metricsTestAt(10, 0)
	expectCounterPeriod(a, aFrom, aP1To)
	a.On("Collect", mock.Anything, mock.Anything, aP1To, aP2To).
		Run(blockUntilDone).Return(metricsTestNoValue, context.DeadlineExceeded)

	b := &MockCounterCollector{name: "b"}
	bFrom := metricsTestAt(9, 0)
	bP1To := metricsTestAt(10, 0)
	expectCounterPeriod(b, bFrom, bP1To)
	expectCounterPeriod(b, bP1To, metricsTestNow)
	collectors := newMetricsTestCollectors(t, repo, nil, nil, []CounterCollector{a, b})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := collectors.CollectMetrics(ctx, models.MetricsCollectionRequest{
		Now:              metricsTestNow,
		SnapshotFrom:     aFrom,
		CounterFrom:      map[string]time.Time{"a": aFrom, "b": bFrom},
		CollectorTimeout: 200 * time.Millisecond,
	})

	require.NoError(t, err)
	require.Len(t, res.Progress, 2)
	pa, pb := res.Progress[0], res.Progress[1]
	assert.Equal(t, "a", pa.CollectorName)
	assert.Equal(t, aP1To, pa.CompletedUntil)
	assert.ErrorIs(t, pa.Err, context.DeadlineExceeded)
	assert.Equal(t, "b", pb.CollectorName)
	assert.Equal(t, metricsTestNow, pb.CompletedUntil)
	assert.NoError(t, pb.Err)
	a.AssertNumberOfCalls(t, "Collect", 2)
	b.AssertNumberOfCalls(t, "Collect", 2)
}
