package worker_jobs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

var (
	metricsTestNow              = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	metricsTestLegacy           = metricsTestNow.Add(-33 * 24 * time.Hour)
	metricsTestFallback         = metricsTestNow.Add(-30 * 24 * time.Hour)
	metricsTestWatermarkA       = models.SpecializedWatermark(models.WatermarkTypeMetrics, "A")
	metricsTestWatermarkB       = models.SpecializedWatermark(models.WatermarkTypeMetrics, "B")
	metricsTestCollectorNames   = []string{"A", "B"}
	metricsTestSendFailedPrefix = "failed to send metrics to ingestion"
)

type metricsCollectionUsecaseMock struct {
	mock.Mock
}

func (m *metricsCollectionUsecaseMock) CounterCollectorNames() []string {
	return m.Called().Get(0).([]string)
}

func (m *metricsCollectionUsecaseMock) CollectMetrics(ctx context.Context,
	req models.MetricsCollectionRequest,
) (models.MetricsCollectionResult, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(models.MetricsCollectionResult), args.Error(1)
}

type metricsWorkerHarness struct {
	worker   MetricCollectionWorker
	usecase  *metricsCollectionUsecaseMock
	repo     *mocks.WatermarkRepository
	requests *atomic.Int32
}

// newMetricsWorkerHarness builds a worker whose ingestion endpoint is a local test server answering
// with the given status, and whose clock is pinned to metricsTestNow.
func newMetricsWorkerHarness(t *testing.T, ingestionStatus int) metricsWorkerHarness {
	t.Helper()

	requests := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(ingestionStatus)
	}))
	t.Cleanup(server.Close)

	config := infra.MetricCollectionConfig{
		MetricsIngestionURL: server.URL,
		FallbackDuration:    30 * 24 * time.Hour,
		JobTimeout:          infra.METRICS_DEFAULT_JOB_TIMEOUT,
		CollectorTimeout:    infra.METRICS_DEFAULT_COLLECTOR_TIMEOUT,
	}

	usecase := &metricsCollectionUsecaseMock{}
	repo := &mocks.WatermarkRepository{}
	executorFactory := executor_factory.NewExecutorFactoryStub()
	transactionFactory := executor_factory.NewTransactionFactoryStub(executorFactory)

	worker := NewMetricCollectionWorker(usecase, repo, executorFactory, transactionFactory, config)
	worker.now = func() time.Time { return metricsTestNow }

	usecase.On("CounterCollectorNames").Return(metricsTestCollectorNames).Maybe()

	return metricsWorkerHarness{
		worker:   worker,
		usecase:  usecase,
		repo:     repo,
		requests: requests,
	}
}

// expectGetWatermark stubs a read of the given watermark type; a nil watermark means none exists.
func (h metricsWorkerHarness) expectGetWatermark(wmType models.WatermarkType, wm *models.Watermark, err error) {
	h.repo.On("GetWatermark", mock.Anything, mock.Anything, mock.Anything, wmType).Return(wm, err).Once()
}

// expectSave stubs a save of the given watermark type at the given time.
func (h metricsWorkerHarness) expectSave(wmType models.WatermarkType, at time.Time, err error) {
	h.repo.On("SaveWatermark", mock.Anything, mock.Anything, mock.Anything, wmType,
		mock.Anything, timeEqualTo(at), mock.Anything).Return(err).Once()
}

func (h metricsWorkerHarness) assertNoSave(t *testing.T) {
	t.Helper()
	h.repo.AssertNotCalled(t, "SaveWatermark", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (h metricsWorkerHarness) assertNoCollection(t *testing.T) {
	t.Helper()
	h.usecase.AssertNotCalled(t, "CollectMetrics", mock.Anything, mock.Anything)
}

func watermarkAt(at time.Time) *models.Watermark {
	return &models.Watermark{WatermarkTime: at}
}

func timeEqualTo(want time.Time) interface{} {
	return mock.MatchedBy(func(got time.Time) bool { return got.Equal(want) })
}

func assertSameInstant(t *testing.T, want, got time.Time, msgAndArgs ...interface{}) {
	t.Helper()
	assert.Truef(t, want.Equal(got), "want %s, got %s %v", want, got, msgAndArgs)
}

// captureRequest stubs CollectMetrics to return the given result and records the request it received.
func (h metricsWorkerHarness) captureRequest(result models.MetricsCollectionResult, captured *models.MetricsCollectionRequest) {
	h.usecase.On("CollectMetrics", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			*captured = args.Get(1).(models.MetricsCollectionRequest)
		}).
		Return(result, nil)
}

func TestMetricCollectionWorker_CounterWithoutOwnWatermarkFallsBackEvenWithLegacy(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusOK)
	h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(metricsTestLegacy), nil)
	h.expectGetWatermark(metricsTestWatermarkA, nil, nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)
	// Without progress, each counter keeps its fallback value
	h.expectSave(metricsTestWatermarkA, metricsTestFallback, nil)
	h.expectSave(metricsTestWatermarkB, metricsTestFallback, nil)
	h.expectSave(models.WatermarkTypeMetrics, metricsTestNow, nil)

	var req models.MetricsCollectionRequest
	h.captureRequest(models.MetricsCollectionResult{}, &req)

	err := h.worker.Work(context.Background(), nil)
	require.NoError(t, err)

	// The legacy watermark (now - 33d) is ignored for counters: without their own watermark they start
	// at the fallback (now - 30d)
	assertSameInstant(t, metricsTestFallback, req.CounterFrom["A"])
	assertSameInstant(t, metricsTestFallback, req.CounterFrom["B"])
	// The snapshot collectors still use the legacy watermark
	assertSameInstant(t, metricsTestLegacy, req.SnapshotFrom)
	h.repo.AssertExpectations(t)
}

func TestMetricCollectionWorker_CounterIgnoresNewerLegacyWatermark(t *testing.T) {
	recentLegacy := metricsTestNow.Add(-2 * 24 * time.Hour)
	h := newMetricsWorkerHarness(t, http.StatusOK)
	h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(recentLegacy), nil)
	h.expectGetWatermark(metricsTestWatermarkA, nil, nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)
	h.expectSave(metricsTestWatermarkA, metricsTestFallback, nil)
	h.expectSave(metricsTestWatermarkB, metricsTestFallback, nil)
	h.expectSave(models.WatermarkTypeMetrics, metricsTestNow, nil)

	var req models.MetricsCollectionRequest
	h.captureRequest(models.MetricsCollectionResult{}, &req)

	err := h.worker.Work(context.Background(), nil)
	require.NoError(t, err)

	// Even a legacy watermark more recent than the fallback is not used for counters
	assertSameInstant(t, metricsTestFallback, req.CounterFrom["A"])
	assertSameInstant(t, metricsTestFallback, req.CounterFrom["B"])
	assertSameInstant(t, recentLegacy, req.SnapshotFrom)
	h.repo.AssertExpectations(t)
}

func TestMetricCollectionWorker_FallsBackWhenNoWatermarkExists(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusOK)
	h.expectGetWatermark(models.WatermarkTypeMetrics, nil, nil)
	h.expectGetWatermark(metricsTestWatermarkA, nil, nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)
	h.expectSave(metricsTestWatermarkA, metricsTestFallback, nil)
	h.expectSave(metricsTestWatermarkB, metricsTestFallback, nil)
	h.expectSave(models.WatermarkTypeMetrics, metricsTestNow, nil)

	var req models.MetricsCollectionRequest
	h.captureRequest(models.MetricsCollectionResult{}, &req)

	err := h.worker.Work(context.Background(), nil)
	require.NoError(t, err)

	assertSameInstant(t, metricsTestFallback, req.SnapshotFrom)
	assertSameInstant(t, metricsTestFallback, req.CounterFrom["A"])
	assertSameInstant(t, metricsTestFallback, req.CounterFrom["B"])
}

func TestMetricCollectionWorker_SpecializedWatermarkWinsOverLegacy(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusOK)
	watermarkA := metricsTestNow.Add(-2 * 24 * time.Hour)
	h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(metricsTestLegacy), nil)
	h.expectGetWatermark(metricsTestWatermarkA, watermarkAt(watermarkA), nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)
	h.expectSave(metricsTestWatermarkA, watermarkA, nil)
	h.expectSave(metricsTestWatermarkB, metricsTestFallback, nil)
	h.expectSave(models.WatermarkTypeMetrics, metricsTestNow, nil)

	var req models.MetricsCollectionRequest
	h.captureRequest(models.MetricsCollectionResult{}, &req)

	err := h.worker.Work(context.Background(), nil)
	require.NoError(t, err)

	// A keeps its own watermark, B has none and falls back to now - FallbackDuration, not to the legacy one
	assertSameInstant(t, watermarkA, req.CounterFrom["A"])
	assertSameInstant(t, metricsTestFallback, req.CounterFrom["B"])
	assertSameInstant(t, metricsTestLegacy, req.SnapshotFrom)
}

func TestMetricCollectionWorker_WatermarkReadErrorFailsJob(t *testing.T) {
	readErr := errors.New("database unavailable")

	t.Run("legacy watermark", func(t *testing.T) {
		h := newMetricsWorkerHarness(t, http.StatusOK)
		h.expectGetWatermark(models.WatermarkTypeMetrics, nil, readErr)

		err := h.worker.Work(context.Background(), nil)

		require.ErrorIs(t, err, readErr)
		h.assertNoCollection(t)
		assert.Equal(t, int32(0), h.requests.Load())
	})

	t.Run("specialized watermark", func(t *testing.T) {
		h := newMetricsWorkerHarness(t, http.StatusOK)
		h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(metricsTestLegacy), nil)
		h.expectGetWatermark(metricsTestWatermarkA, nil, readErr)

		err := h.worker.Work(context.Background(), nil)

		require.ErrorIs(t, err, readErr)
		h.assertNoCollection(t)
		assert.Equal(t, int32(0), h.requests.Load())
	})
}

func TestMetricCollectionWorker_SendFailureSkipsWatermarkSave(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusInternalServerError)
	h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(metricsTestLegacy), nil)
	h.expectGetWatermark(metricsTestWatermarkA, nil, nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)

	var req models.MetricsCollectionRequest
	h.captureRequest(models.MetricsCollectionResult{
		Progress: []models.MetricsCounterProgress{
			{CollectorName: "A", From: metricsTestFallback, CompletedUntil: metricsTestNow},
		},
	}, &req)

	err := h.worker.Work(context.Background(), nil)

	require.Error(t, err)
	assert.ErrorContains(t, err, metricsTestSendFailedPrefix)
	assert.Equal(t, int32(3), h.requests.Load(), "the send is retried 3 times")
	h.assertNoSave(t)
}

func TestMetricCollectionWorker_PartialProgressSavesEachCollector(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusOK)
	h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(metricsTestLegacy), nil)
	h.expectGetWatermark(metricsTestWatermarkA, nil, nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)

	completedA := metricsTestNow.Add(-10 * 24 * time.Hour)
	h.expectSave(metricsTestWatermarkA, completedA, nil)
	h.expectSave(metricsTestWatermarkB, metricsTestNow, nil)
	h.expectSave(models.WatermarkTypeMetrics, metricsTestNow, nil)

	var req models.MetricsCollectionRequest
	h.captureRequest(models.MetricsCollectionResult{
		Progress: []models.MetricsCounterProgress{
			{CollectorName: "A", From: metricsTestFallback, CompletedUntil: completedA,
				Err: errors.New("collector timed out")},
			{CollectorName: "B", From: metricsTestFallback, CompletedUntil: metricsTestNow},
		},
	}, &req)

	err := h.worker.Work(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, int32(1), h.requests.Load())
	h.repo.AssertExpectations(t)
	h.repo.AssertNumberOfCalls(t, "SaveWatermark", 3)
}

func TestMetricCollectionWorker_NoProgressSavesFallbackValue(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusOK)
	h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(metricsTestLegacy), nil)
	h.expectGetWatermark(metricsTestWatermarkA, nil, nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)

	// Collector A completed no period: its CompletedUntil stays at its fallback value.
	// Collector B has no progress at all: it also keeps its fallback value.
	h.expectSave(metricsTestWatermarkA, metricsTestFallback, nil)
	h.expectSave(metricsTestWatermarkB, metricsTestFallback, nil)
	h.expectSave(models.WatermarkTypeMetrics, metricsTestNow, nil)

	var req models.MetricsCollectionRequest
	h.captureRequest(models.MetricsCollectionResult{
		Progress: []models.MetricsCounterProgress{
			{CollectorName: "A", From: metricsTestFallback, CompletedUntil: metricsTestFallback},
		},
	}, &req)

	err := h.worker.Work(context.Background(), nil)

	require.NoError(t, err)
	h.repo.AssertExpectations(t)
}

func TestMetricCollectionWorker_SendAndSaveSurviveJobCancellation(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusOK)
	h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(metricsTestLegacy), nil)
	h.expectGetWatermark(metricsTestWatermarkA, nil, nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)

	// The save context is inspected at call time, because the repo mock ignores it afterwards.
	type saveCall struct {
		ctxErr      error
		deadline    time.Time
		hasDeadline bool
	}
	var saves []saveCall
	recordSave := func(args mock.Arguments) {
		ctx := args.Get(0).(context.Context)
		deadline, hasDeadline := ctx.Deadline()
		saves = append(saves, saveCall{ctxErr: ctx.Err(), deadline: deadline, hasDeadline: hasDeadline})
	}

	completedA := metricsTestNow.Add(-time.Hour)
	h.repo.On("SaveWatermark", mock.Anything, mock.Anything, mock.Anything, metricsTestWatermarkA,
		mock.Anything, timeEqualTo(completedA), mock.Anything).Run(recordSave).Return(nil).Once()
	h.repo.On("SaveWatermark", mock.Anything, mock.Anything, mock.Anything, metricsTestWatermarkB,
		mock.Anything, timeEqualTo(metricsTestFallback), mock.Anything).Run(recordSave).Return(nil).Once()
	h.repo.On("SaveWatermark", mock.Anything, mock.Anything, mock.Anything, models.WatermarkTypeMetrics,
		mock.Anything, timeEqualTo(metricsTestNow), mock.Anything).Run(recordSave).Return(nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.usecase.On("CollectMetrics", mock.Anything, mock.Anything).
		Run(func(_ mock.Arguments) {
			// The job is cancelled while the collection is running
			cancel()
		}).
		Return(models.MetricsCollectionResult{
			Progress: []models.MetricsCounterProgress{
				{CollectorName: "A", From: metricsTestFallback, CompletedUntil: completedA},
			},
		}, nil)

	before := time.Now()
	err := h.worker.Work(ctx, nil)

	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.NoError(t, err)
	assert.Equal(t, int32(1), h.requests.Load(), "the send must not be cancelled with the job")

	require.Len(t, saves, 3)
	for _, s := range saves {
		assert.NoError(t, s.ctxErr, "the save context must not be cancelled with the job")
		require.True(t, s.hasDeadline, "the save context must carry its own deadline")
		assert.WithinDuration(t, before.Add(infra.METRICS_SAVE_TIMEOUT), s.deadline, 2*time.Second)
	}
	h.repo.AssertExpectations(t)
}

func TestMetricCollectionWorker_FutureWatermarkIsClampedToNow(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusOK)
	h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(metricsTestLegacy), nil)
	h.expectGetWatermark(metricsTestWatermarkA, watermarkAt(metricsTestNow.Add(time.Hour)), nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)
	h.expectSave(metricsTestWatermarkA, metricsTestNow, nil)
	h.expectSave(metricsTestWatermarkB, metricsTestFallback, nil)
	h.expectSave(models.WatermarkTypeMetrics, metricsTestNow, nil)

	var req models.MetricsCollectionRequest
	h.captureRequest(models.MetricsCollectionResult{}, &req)

	require.NoError(t, h.worker.Work(context.Background(), nil))

	assertSameInstant(t, metricsTestNow, req.CounterFrom["A"])
	assertSameInstant(t, metricsTestFallback, req.CounterFrom["B"])
}

func TestMetricCollectionWorker_ZeroWatermarkTimeFallsBack(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusOK)
	h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(metricsTestLegacy), nil)
	h.expectGetWatermark(metricsTestWatermarkA, watermarkAt(time.Time{}), nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)
	h.expectSave(metricsTestWatermarkA, metricsTestFallback, nil)
	h.expectSave(metricsTestWatermarkB, metricsTestFallback, nil)
	h.expectSave(models.WatermarkTypeMetrics, metricsTestNow, nil)

	var req models.MetricsCollectionRequest
	h.captureRequest(models.MetricsCollectionResult{}, &req)

	require.NoError(t, h.worker.Work(context.Background(), nil))

	// A zero watermark time is treated like a missing row: the counter falls back, the legacy watermark is not used
	assertSameInstant(t, metricsTestFallback, req.CounterFrom["A"])
	assertSameInstant(t, metricsTestFallback, req.CounterFrom["B"])
}

func TestMetricCollectionWorker_CollectionBudgetDeadlineAndTimeout(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusOK)
	h.expectGetWatermark(models.WatermarkTypeMetrics, nil, nil)
	h.expectGetWatermark(metricsTestWatermarkA, nil, nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)
	h.expectSave(metricsTestWatermarkA, metricsTestFallback, nil)
	h.expectSave(metricsTestWatermarkB, metricsTestFallback, nil)
	h.expectSave(models.WatermarkTypeMetrics, metricsTestNow, nil)

	var deadline time.Time
	var hasDeadline bool
	h.usecase.On("CollectMetrics", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			deadline, hasDeadline = args.Get(0).(context.Context).Deadline()
		}).
		Return(models.MetricsCollectionResult{}, nil)

	budget := h.worker.config.CollectionBudget()

	before := time.Now()
	err := h.worker.Work(context.Background(), nil)
	require.NoError(t, err)

	require.True(t, hasDeadline, "the collection context must carry a deadline")
	assert.WithinDuration(t, before.Add(budget), deadline, time.Second)
	assert.Equal(t, infra.METRICS_DEFAULT_JOB_TIMEOUT, h.worker.Timeout(nil))
}

func TestMetricCollectionWorker_CollectionErrorSkipsSendAndSave(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusOK)
	h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(metricsTestLegacy), nil)
	h.expectGetWatermark(metricsTestWatermarkA, nil, nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)

	collectErr := errors.New("collection exploded")
	h.usecase.On("CollectMetrics", mock.Anything, mock.Anything).
		Return(models.MetricsCollectionResult{}, collectErr)

	err := h.worker.Work(context.Background(), nil)

	require.ErrorIs(t, err, collectErr)
	assert.Equal(t, int32(0), h.requests.Load())
	h.assertNoSave(t)
}

func TestMetricCollectionWorker_WatermarkSaveFailureFailsJob(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusOK)
	h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(metricsTestLegacy), nil)
	h.expectGetWatermark(metricsTestWatermarkA, nil, nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)

	saveErr := errors.New("transaction failed")
	completedA := metricsTestNow.Add(-time.Hour)
	h.expectSave(metricsTestWatermarkA, completedA, saveErr)

	var req models.MetricsCollectionRequest
	h.captureRequest(models.MetricsCollectionResult{
		Progress: []models.MetricsCounterProgress{
			{CollectorName: "A", From: metricsTestFallback, CompletedUntil: completedA},
		},
	}, &req)

	err := h.worker.Work(context.Background(), nil)

	require.ErrorIs(t, err, saveErr)
	assert.Equal(t, int32(1), h.requests.Load(), "the metrics were sent before the save failed")
	// The failing save aborts the transaction: the global watermark is never attempted
	h.repo.AssertNumberOfCalls(t, "SaveWatermark", 1)
}

func TestMetricCollectionWorker_CounterMissingFromProgressIsSavedAtItsFallback(t *testing.T) {
	h := newMetricsWorkerHarness(t, http.StatusOK)
	h.expectGetWatermark(models.WatermarkTypeMetrics, watermarkAt(metricsTestLegacy), nil)
	h.expectGetWatermark(metricsTestWatermarkA, watermarkAt(metricsTestNow.Add(-2*24*time.Hour)), nil)
	h.expectGetWatermark(metricsTestWatermarkB, nil, nil)

	// A has progress, B is missing from the orchestrator result
	completedA := metricsTestNow.Add(-24 * time.Hour)
	h.expectSave(metricsTestWatermarkA, completedA, nil)
	h.expectSave(metricsTestWatermarkB, metricsTestFallback, nil)
	h.expectSave(models.WatermarkTypeMetrics, metricsTestNow, nil)

	var req models.MetricsCollectionRequest
	h.usecase.On("CollectMetrics", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			req = args.Get(1).(models.MetricsCollectionRequest)
		}).
		Return(models.MetricsCollectionResult{
			Progress: []models.MetricsCounterProgress{
				{CollectorName: "A", From: metricsTestNow.Add(-2 * 24 * time.Hour), CompletedUntil: completedA},
			},
		}, nil)

	require.NoError(t, h.worker.Work(context.Background(), nil))

	// The orchestrator was asked for both counters, but only reported progress for A
	require.Contains(t, req.CounterFrom, "A")
	require.Contains(t, req.CounterFrom, "B")
	assertSameInstant(t, metricsTestFallback, req.CounterFrom["B"])
	h.repo.AssertExpectations(t)
	h.repo.AssertNumberOfCalls(t, "SaveWatermark", 3)
}
