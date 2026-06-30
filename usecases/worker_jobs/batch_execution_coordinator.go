package worker_jobs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/decision_phantom"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/twpayne/go-geos"
	"golang.org/x/sync/errgroup"
)

const (
	// batchExecBatchSize is the number of decisions evaluated concurrently in a batch execution.
	batchExecBatchSize = 20

	// batchExecCommitBatchSize is the number of object ids read from the manifest per loop iteration.
	// They are evaluated concurrently and the surviving decisions are inserted in a single
	// transaction together with the manifest cursor advance. Kept small to bound the number of
	// rows written per transaction.
	batchExecCommitBatchSize = 50

	// batchExecPerIterTimeout caps a single loop iteration so a wedged DB/blob/screening
	// call cannot hang the coordinator until the whole-job timeout. A timeout is retryable.
	batchExecPerIterTimeout = 1 * time.Minute

	// batchExecDefaultRunDuration is the wall-clock budget for a whole run when no deadline
	// was recorded at setup. The deadline is the only termination on sustained failure.
	batchExecDefaultRunDuration = 9 * time.Minute

	scheduledExecutionFullMaxRuntime = 24 * time.Hour

	// Coordinator job timeout: must comfortably exceed the run deadline so River's stuck-job
	// rescuer never double-runs a healthy coordinator.
	batchExecJobTimeout = batchExecDefaultRunDuration + batchExecPerIterTimeout + time.Second*20

	batchExecBackoffBase  = 5 * time.Second
	batchExecBackoffCap   = 2 * time.Minute
	batchExecSentryEveryN = 20
)

type batchCoordinatorRepository interface {
	GetScheduledExecution(ctx context.Context, exec repositories.Executor, id string) (models.ScheduledExecution, error)
	UpdateScheduledExecutionStatus(
		ctx context.Context,
		exec repositories.Executor,
		input models.UpdateScheduledExecutionStatusInput,
	) error
	AdvanceScheduledExecutionManifest(
		ctx context.Context,
		exec repositories.Executor,
		input models.AdvanceScheduledExecutionManifestInput,
	) error
	GetAnalyticsSettings(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) (map[string]analytics.Settings, error)
}

type BatchExecutionCoordinator struct {
	repository                 batchCoordinatorRepository
	executorFactory            executor_factory.ExecutorFactory
	transactionFactory         executor_factory.TransactionFactory
	dataModelRepository        repositories.DataModelRepository
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	decisionRepository         repositories.DecisionRepository
	offloadedReader            repositories.OffloadedReadWriter
	blobRepository             repositories.BlobRepository
	manifestBucketUrl          string
	webhookEventsSender        webhookEventsUsecase
	scenarioFetcher            scenarios.ScenarioFetcher
	phantomDecision            decision_phantom.PhantomDecisionUsecase
	scenarioEvaluator          ScenarioEvaluator
	screeningRepository        decisionWorkerScreeningWriter
	taskQueueRepository        repositories.TaskQueueRepository
}

func NewBatchExecutionCoordinator(
	repository batchCoordinatorRepository,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	dataModelRepository repositories.DataModelRepository,
	ingestedDataReadRepository repositories.IngestedDataReadRepository,
	decisionRepository repositories.DecisionRepository,
	offloadedReader repositories.OffloadedReadWriter,
	blobRepository repositories.BlobRepository,
	manifestBucketUrl string,
	webhookEventsSender webhookEventsUsecase,
	scenarioFetcher scenarios.ScenarioFetcher,
	phantom decision_phantom.PhantomDecisionUsecase,
	scenarioEvaluator ScenarioEvaluator,
	screeningRepository decisionWorkerScreeningWriter,
	taskQueueRepository repositories.TaskQueueRepository,
) BatchExecutionCoordinator {
	return BatchExecutionCoordinator{
		repository:                 repository,
		executorFactory:            executorFactory,
		transactionFactory:         transactionFactory,
		dataModelRepository:        dataModelRepository,
		ingestedDataReadRepository: ingestedDataReadRepository,
		decisionRepository:         decisionRepository,
		offloadedReader:            offloadedReader,
		blobRepository:             blobRepository,
		manifestBucketUrl:          manifestBucketUrl,
		webhookEventsSender:        webhookEventsSender,
		scenarioFetcher:            scenarioFetcher,
		phantomDecision:            phantom,
		scenarioEvaluator:          scenarioEvaluator,
		screeningRepository:        screeningRepository,
		taskQueueRepository:        taskQueueRepository,
	}
}

// batchInvariants holds the per-run context (scenario, data model, pivots, client DB handle)
// loaded once and reused for every object in every batch, rather than reloaded per object.
type batchInvariants struct {
	scenario             models.Scenario
	scenarioIterationId  string
	dataModel            models.DataModel
	table                models.Table
	pivots               []models.Pivot
	clientDb             repositories.Executor
	scheduledExecutionId string
}

// evalOutcome is the result of evaluating one object, outside any transaction.
type evalOutcome struct {
	objectId          string
	triggerPassed     bool
	skipped           bool // object not found in the table; counts as evaluated, not created
	scenarioExecution models.ScenarioExecution
	evalParams        evaluate_scenario.ScenarioEvaluationParameters
	object            models.ClientObject
	error             error
}

type BatchExecutionCoordinatorWorker struct {
	river.WorkerDefaults[models.BatchExecutionCoordinatorArgs]
	coordinator *BatchExecutionCoordinator
}

func NewBatchExecutionCoordinatorWorker(coordinator *BatchExecutionCoordinator) *BatchExecutionCoordinatorWorker {
	return &BatchExecutionCoordinatorWorker{coordinator: coordinator}
}

func (w *BatchExecutionCoordinatorWorker) Timeout(job *river.Job[models.BatchExecutionCoordinatorArgs]) time.Duration {
	return batchExecJobTimeout
}

func (w *BatchExecutionCoordinatorWorker) Work(ctx context.Context, job *river.Job[models.BatchExecutionCoordinatorArgs]) error {
	return w.coordinator.Run(ctx, job.Args.ScheduledExecutionId)
}

// Run drives the scheduled execution to completion (or to its deadline). It returns an error
// only when the run should be retried by River as a whole (e.g. process shutdown mid-run, so
// it resumes from the committed offset). Terminal outcomes (success, cancellation, deadline,
// hard failure) set the final status themselves and return nil.
func (c *BatchExecutionCoordinator) Run(ctx context.Context, scheduledExecutionId string) error {
	logger := utils.LoggerFromContext(ctx).With("scheduled_execution_id", scheduledExecutionId)
	ctx = utils.StoreLoggerInContext(ctx, logger)
	exec := c.executorFactory.NewExecutor()

	setupCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	se, err := c.repository.GetScheduledExecution(setupCtx, exec, scheduledExecutionId)
	if err != nil {
		return errors.Wrap(err, "could not load scheduled execution in batch coordinator")
	}
	if se.ManifestBlobKey == nil {
		// Invariant: the coordinator only ever runs for v2 executions, which always have a
		// manifest. Mark failed rather than spin.
		return c.markFailed(ctx, se.Id, errors.New("batch coordinator started for an execution without a manifest"))
	}

	if se.Deadline == nil {
		return c.markFailed(ctx, se.Id, errors.New("invariant violated: job scheduled for v2 batch execution must have a deadline set"))
	}

	// A context timeout during the job execution leads to a snooze and retry, the next execution can mark it as failed if the deadline is passed
	if time.Now().After(*se.Deadline) {
		if se.ManifestRowsProcessed >= int64(*se.NumberOfPlannedDecisions) {
			return c.repository.UpdateScheduledExecutionStatus(ctx, exec, models.UpdateScheduledExecutionStatusInput{Id: se.Id, Status: models.ScheduledExecutionSuccess})
		} else {
			logger.InfoContext(ctx, "Scheduled execution not completed after the deadline, marking as failed")
			return c.markFailed(ctx, se.Id, errors.New("Scheduled execution not completed after the deadline"))
		}
	}

	inv, err := c.loadInvariants(setupCtx, exec, se)
	if err != nil {
		return errors.Wrap(err, "could not load batch invariants")
	}

	ctx, cancel = context.WithTimeout(ctx, batchExecDefaultRunDuration)
	defer cancel()

	var planned int64
	if se.NumberOfPlannedDecisions != nil {
		planned = int64(*se.NumberOfPlannedDecisions)
	}

	offset := se.ManifestByteOffset
	rows := se.ManifestRowsProcessed
	created := se.NumberOfCreatedDecisions
	evaluated := se.NumberOfEvaluatedDecisions
	consecutiveFailures := 0

	// Keep a single manifest reader open across the slice and read batches sequentially from it,
	// rather than reopening the blob for every batch. The reader is reopened only when it is nil:
	// at slice start, and after any error that leaves the offset unadvanced (the reader has by
	// then consumed bytes past the committed offset, so it must be re-seeked there).
	var manifestCloser io.Closer
	var manifestReader *bufio.Reader
	defer func() {
		if manifestCloser != nil {
			_ = manifestCloser.Close()
		}
	}()
	resetManifestReader := func() {
		if manifestCloser != nil {
			_ = manifestCloser.Close()
			manifestCloser = nil
		}
		manifestReader = nil
	}
	openManifestReader := func() error {
		blob, err := c.blobRepository.GetBlob(ctx, c.manifestBucketUrl, *se.ManifestBlobKey,
			repositories.WithBeginOffset(offset))
		if err != nil {
			return errors.Wrapf(err, "could not open manifest %s at offset %d", *se.ManifestBlobKey, offset)
		}
		manifestCloser = blob.ReadCloser
		manifestReader = bufio.NewReader(blob.ReadCloser)
		return nil
	}

	for {
		if ctx.Err() != nil {
			logger.InfoContext(ctx, fmt.Sprintf("Per-job timeout of %s reached for scheduled execution coordinator job, snoozing 5 sec before reexecuting", batchExecDefaultRunDuration))
			return river.JobSnooze(time.Second * 5)
		}

		// Re-read status before each batch so a cancellation (status flipped away from processing) stops the run promptly.
		current, err := c.repository.GetScheduledExecution(ctx, exec, scheduledExecutionId)
		if err != nil {
			c.logAndSleepWithBackoff(ctx, err, &consecutiveFailures)
			continue
		}
		if current.Status != models.ScheduledExecutionProcessing {
			logger.InfoContext(ctx, fmt.Sprintf(
				"batch execution no longer processing (status %s), coordinator exiting", current.Status))
			return nil
		}

		if rows >= planned {
			logger.InfoContext(ctx, fmt.Sprintf("batch execution complete: %d decisions created, %d evaluated", created, evaluated))
			return c.repository.UpdateScheduledExecutionStatus(ctx, exec, models.UpdateScheduledExecutionStatusInput{Id: se.Id, Status: models.ScheduledExecutionSuccess})
		}

		if manifestReader == nil {
			if err := openManifestReader(); err != nil {
				c.logAndSleepWithBackoff(ctx, err, &consecutiveFailures)
				continue
			}
		}

		ids, consumed, err := readManifestBatch(manifestReader, batchExecCommitBatchSize)
		if err != nil {
			resetManifestReader()
			c.logAndSleepWithBackoff(ctx, err, &consecutiveFailures)
			continue
		}
		if len(ids) == 0 {
			// Manifest drained earlier than the planned count predicted. Finalize rather than
			// loop forever; log because counts should have matched.
			logger.WarnContext(ctx, fmt.Sprintf(
				"manifest exhausted at %d rows but %d were planned; finalizing", rows, planned))
			return c.repository.UpdateScheduledExecutionStatus(ctx, exec, models.UpdateScheduledExecutionStatusInput{Id: se.Id, Status: models.ScheduledExecutionSuccess})
		}
		newOffset := offset + consumed

		// One per-iteration context bounds evaluation and the persistence transaction — so no
		// single wedged operation hangs the coordinator until the whole-job timeout. Derived from
		// the job ctx so a deploy still propagates cancellation. A timeout here is retryable.
		batchCtx, cancel := context.WithTimeout(ctx, batchExecPerIterTimeout)

		results, retryErr := c.evaluateBatch(batchCtx, inv, ids)
		if retryErr != nil {
			cancel()
			// Offset is not advanced, so the reader (now past this batch) must re-seek to it.
			resetManifestReader()
			c.logAndSleepWithBackoff(ctx, retryErr, &consecutiveFailures)
			continue
		}

		batchCreated := 0
		batchEvaluated := len(results)
		var webhookIds []string
		var callbacks []func()

		err = c.transactionFactory.Transaction(batchCtx, func(tx repositories.Transaction) error {
			// Reset accumulators so a retry of the same batch (tx rollback) starts clean.
			batchCreated = 0
			webhookIds = nil
			callbacks = nil
			for _, r := range results {
				if r.skipped || !r.triggerPassed {
					if cb := c.testRunCallback(ctx, inv, r, false); cb != nil {
						callbacks = append(callbacks, cb)
					}
					continue
				}
				wid, err := c.storeDecision(batchCtx, tx, inv, r)
				if err != nil {
					return err
				}
				webhookIds = append(webhookIds, wid...)
				if cb := c.testRunCallback(ctx, inv, r, true); cb != nil {
					callbacks = append(callbacks, cb)
				}
				batchCreated++
			}

			return c.repository.AdvanceScheduledExecutionManifest(batchCtx, tx, models.AdvanceScheduledExecutionManifestInput{
				Id:                         scheduledExecutionId,
				ManifestByteOffset:         newOffset,
				ManifestRowsProcessed:      rows + int64(len(ids)),
				NumberOfCreatedDecisions:   created + batchCreated,
				NumberOfEvaluatedDecisions: evaluated + batchEvaluated,
			})
		})
		cancel()
		if err != nil {
			// Store or advance failed: nothing committed, do not advance our cursors. The reader
			// has consumed this batch, so re-seek it to the committed offset and retry.
			resetManifestReader()
			c.logAndSleepWithBackoff(ctx, err, &consecutiveFailures)
			continue
		}

		// Committed. Advance the in-memory cursors, fire post-commit side effects, reset backoff.
		offset = newOffset
		rows += int64(len(ids))
		created += batchCreated
		evaluated += batchEvaluated
		consecutiveFailures = 0

		for _, wid := range webhookIds {
			c.webhookEventsSender.SendWebhookEventAsync(ctx, wid)
		}
		for _, cb := range callbacks {
			cb()
		}
	}
}

func (c *BatchExecutionCoordinator) loadInvariants(
	ctx context.Context,
	exec repositories.Executor,
	se models.ScheduledExecution,
) (batchInvariants, error) {
	scenarioAndIteration, err := c.scenarioFetcher.FetchScenarioAndIteration(ctx, exec, se.ScenarioIterationId)
	if err != nil {
		return batchInvariants{}, err
	}
	scenario := scenarioAndIteration.Scenario

	dataModel, err := c.dataModelRepository.GetDataModel(ctx, exec, scenario.OrganizationId, false, true)
	if err != nil {
		return batchInvariants{}, err
	}
	table, ok := dataModel.Tables[scenario.TriggerObjectType]
	if !ok {
		err = c.repository.UpdateScheduledExecutionStatus(ctx, exec, models.UpdateScheduledExecutionStatusInput{
			Id:     se.Id,
			Status: models.ScheduledExecutionFailure,
		})
		if err != nil {
			utils.LogAndReportSentryError(ctx, err)
		}
		return batchInvariants{}, river.JobCancel(err)
	}

	pivotsMeta, err := c.dataModelRepository.ListPivots(ctx, exec, scenario.OrganizationId, nil, true, false)
	if err != nil {
		return batchInvariants{}, err
	}
	pivots := models.FindPivotsForTable(pivotsMeta, scenario.TriggerObjectType, dataModel)

	clientDb, err := c.executorFactory.NewClientDbExecutor(ctx, scenario.OrganizationId)
	if err != nil {
		return batchInvariants{}, err
	}

	return batchInvariants{
		scenario:             scenario,
		scenarioIterationId:  se.ScenarioIterationId,
		dataModel:            dataModel,
		table:                table,
		pivots:               pivots,
		clientDb:             clientDb,
		scheduledExecutionId: se.Id,
	}, nil
}

// readManifestBatch reads up to batchSize object ids from reader and returns them along with
// the number of bytes consumed, so the caller can advance its byte offset. Each manifest line
// is a Go-quoted id (see StreamAllObjectIdsFromTable); reads stop at EOF or batchSize.
func readManifestBatch(reader *bufio.Reader, batchSize int) (ids []string, consumed int64, err error) {
	ids = make([]string, 0, batchSize)
	for len(ids) < batchSize {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			consumed += int64(len(line))
			if trimmed := strings.TrimRight(line, "\n"); trimmed != "" {
				id, unquoteErr := strconv.Unquote(trimmed)
				if unquoteErr != nil {
					return nil, 0, errors.Wrapf(unquoteErr, "corrupt manifest line %q", trimmed)
				}
				ids = append(ids, id)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, 0, errors.Wrap(readErr, "error reading manifest")
		}
	}

	return ids, consumed, nil
}

func (c *BatchExecutionCoordinator) evaluateBatch(
	ctx context.Context,
	inv batchInvariants,
	ids []string,
) (results []evalOutcome, err error) {
	batchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	outcomes := make([]evalOutcome, len(ids))
	var g errgroup.Group
	g.SetLimit(batchExecBatchSize)
	for i, id := range ids {
		g.Go(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					outcomes[i] = evalOutcome{objectId: id, error: errors.Newf("panic evaluating object %s: %v", id, r)}
				}
			}()
			outcomes[i] = c.evaluateObject(batchCtx, inv, id)
			if outcomes[i].error != nil {
				// on any error, cancel the context. The whole batch is retried
				cancel()
			}
			return nil
		})
	}
	_ = g.Wait()

	for _, o := range outcomes {
		switch {
		case o.error != nil:
			// any error will cause some more context canceled errors because they cancel the context -- preferably keep the initial error and return it
			if err == nil || !(errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				err = o.error
			}
		default:
			results = append(results, o)
		}
	}

	if err != nil {
		return nil, err
	}
	return results, nil
}

func (c *BatchExecutionCoordinator) evaluateObject(
	ctx context.Context,
	inv batchInvariants,
	objectId string,
) evalOutcome {
	objectMap, err := c.ingestedDataReadRepository.QueryIngestedObject(ctx, inv.clientDb, inv.table, objectId)
	if err != nil {
		return evalOutcome{objectId: objectId, error: errors.Wrapf(err, "error querying ingested object %s", objectId)}
	}
	if len(objectMap) == 0 {
		utils.LogAndReportSentryError(ctx, errors.Newf("object %s not found in table %s", objectId, inv.table.Name))
		return evalOutcome{objectId: objectId, skipped: true}
	}

	// Scheduled executions are read from the database whereas the engine was built for JSON
	// input; untype special types (geolocations, IP addresses, metadata fields).
	for idx := range objectMap {
		for k, v := range objectMap[idx].Data {
			if strings.Contains(k, ".metadata") {
				delete(objectMap[idx].Data, k)
				continue
			}
			switch typed := v.(type) {
			case *geos.Geom:
				objectMap[idx].Data[k] = fmt.Sprintf("%f,%f", typed.Y(), typed.X())
			case netip.Prefix:
				objectMap[idx].Data[k] = typed.Addr().String()
			}
		}
	}

	object := models.ClientObject{TableName: inv.table.Name, Data: objectMap[0].Data}
	evalParams := evaluate_scenario.ScenarioEvaluationParameters{
		Scenario:          inv.scenario,
		TargetIterationId: &inv.scenarioIterationId,
		ClientObject:      object,
		DataModel:         inv.dataModel,
		Pivots:            inv.pivots,
	}

	triggerPassed, scenarioExecution, err := c.scenarioEvaluator.EvalScenario(ctx, evalParams)
	if err != nil {
		return evalOutcome{objectId: objectId, error: errors.Wrapf(err, "error evaluating scenario for object %s", objectId)}
	}

	return evalOutcome{
		objectId:          objectId,
		triggerPassed:     triggerPassed,
		scenarioExecution: scenarioExecution,
		evalParams:        evalParams,
		object:            object,
	}
}

// storeDecision persists one evaluated decision and its side records inside the batch tx.
func (c *BatchExecutionCoordinator) storeDecision(
	ctx context.Context,
	tx repositories.Transaction,
	inv batchInvariants,
	o evalOutcome,
) ([]string, error) {
	decision := models.AdaptScenarExecToDecision(o.scenarioExecution, o.object, &inv.scheduledExecutionId)

	analyticsFields := c.scenarioEvaluator.GetDataAccessor(o.evalParams).
		GetAnalyticsFields(ctx, tx, c.repository, o.evalParams)

	err := c.decisionRepository.StoreDecision(
		ctx,
		tx,
		c.offloadedReader,
		decision,
		inv.scenario.OrganizationId,
		decision.DecisionId.String(),
		analyticsFields,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error storing decision in batch coordinator")
	}

	for _, sce := range decision.ScreeningExecutions {
		matchesToInsert, offloadErr := c.offloadedReader.OffloadScreeningMatches(ctx, sce)
		if offloadErr != nil {
			return nil, errors.Wrap(offloadErr, "could not offload screening match payloads in batch coordinator")
		}
		sce.Matches = matchesToInsert
		if err := c.screeningRepository.InsertScreening(ctx, tx, sce); err != nil {
			return nil, errors.Wrap(err, "error storing screening execution in batch coordinator")
		}
	}

	webhookEventId := pure_utils.NewId().String()
	err = c.webhookEventsSender.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
		Id:             webhookEventId,
		OrganizationId: decision.OrganizationId,
		EventContent:   models.NewWebhookEventDecisionCreated(decision),
	})
	if err != nil {
		return nil, err
	}

	err = c.taskQueueRepository.EnqueueDecisionWorkflowTask(ctx, tx, decision.OrganizationId, decision.DecisionId.String())
	if err != nil {
		return nil, err
	}

	return []string{webhookEventId}, nil
}

// testRunCallback returns a function, to be run after the batch commits, that evaluates a
// phantom decision against any active test run for the scenario. Returns nil when there is
// nothing to evaluate (the object was not found).
func (c *BatchExecutionCoordinator) testRunCallback(
	ctx context.Context,
	inv batchInvariants,
	o evalOutcome,
	triggered bool,
) func() {
	if o.skipped {
		return nil
	}
	return func() {
		evalParams := o.evalParams
		evalParams.TargetIterationId = nil
		if triggered {
			evalParams.CachedScreenings = pure_utils.MapSliceToMap(
				o.scenarioExecution.ScreeningExecutions,
				func(scm models.ScreeningWithMatches) (string, models.ScreeningWithMatches) {
					return o.scenarioExecution.ScenarioIterationId.String(), scm
				},
			)
		}
		phantomInput := models.CreatePhantomDecisionInput{
			OrganizationId: inv.scenario.OrganizationId,
			Scenario:       inv.scenario,
			ClientObject:   o.object,
		}
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		logger := utils.LoggerFromContext(ctx).With("phantom_decisions_with_scenario_id", phantomInput.Scenario.Id)
		if _, _, errPhantom := c.phantomDecision.CreatePhantomDecision(ctx, phantomInput, evalParams); errPhantom != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("Error when creating phantom decisions with scenario id %s: %s",
				phantomInput.Scenario.Id, errPhantom.Error()))
		}
	}
}

func (c *BatchExecutionCoordinator) markFailed(
	ctx context.Context,
	id string,
	err error,
) error {
	repoErr := c.repository.UpdateScheduledExecutionStatus(ctx, c.executorFactory.NewExecutor(), models.UpdateScheduledExecutionStatusInput{
		Id:     id,
		Status: models.ScheduledExecutionFailure,
	})
	if repoErr != nil {
		utils.LogAndReportSentryError(ctx, repoErr)
	}
	return river.JobCancel(err)
}

// logAndSleepWithBackoff keeps the run in the loop on a presumed-retryable error: log every time,
// report to Sentry throttled, and back off (ctx-aware) before retrying the same offset. The
// deadline check is what eventually terminates a run that never recovers.
func (c *BatchExecutionCoordinator) logAndSleepWithBackoff(ctx context.Context, err error, consecutiveFailures *int) {
	// return early on canceled context, it is handled at the beginning of the loop
	if ctx.Err() != nil {
		return
	}
	*consecutiveFailures++
	logger := utils.LoggerFromContext(ctx)
	logger.ErrorContext(ctx, fmt.Sprintf("retryable error in batch coordinator (consecutive: %d): %s",
		*consecutiveFailures, err.Error()))

	if *consecutiveFailures == 1 || *consecutiveFailures%batchExecSentryEveryN == 0 {
		utils.LogAndReportSentryError(ctx, err)
	}

	delay := batchExecBackoffDelay(*consecutiveFailures)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func batchExecBackoffDelay(consecutiveFailures int) time.Duration {
	if consecutiveFailures < 1 {
		consecutiveFailures = 1
	}
	delay := batchExecBackoffBase
	for i := 1; i < consecutiveFailures && delay < batchExecBackoffCap; i++ {
		delay *= 2
	}
	if delay > batchExecBackoffCap {
		delay = batchExecBackoffCap
	}
	return delay
}
