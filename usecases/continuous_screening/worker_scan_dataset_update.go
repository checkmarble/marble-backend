package continuous_screening

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"golang.org/x/sync/errgroup"
)

const MaxConcurrentDatasetUpdates = 3

type scanDatasetUpdatesWorkerRepository interface {
	GetLastProcessedVersion(
		ctx context.Context,
		exec repositories.Executor,
		datasetName string,
	) (models.ContinuousScreeningDatasetUpdate, error)
	CreateContinuousScreeningDatasetUpdate(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreeningDatasetUpdate,
	) (models.ContinuousScreeningDatasetUpdate, error)
	CreateContinuousScreeningUpdateJob(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreeningUpdateJob,
	) (models.ContinuousScreeningUpdateJob, error)
	ListContinuousScreeningConfigs(
		ctx context.Context,
		exec repositories.Executor,
	) ([]models.ContinuousScreeningConfig, error)
}

type scanDatasetUpdatesWorkerScreeningProvider interface {
	GetRawCatalog(ctx context.Context) (models.OpenSanctionsRawCatalog, error)
}

type scanDatasetUpdatesWorkerTaskEnqueuer interface {
	EnqueueContinuousScreeningApplyDeltaFileTask(
		ctx context.Context,
		tx repositories.Transaction,
		orgId uuid.UUID,
		updateId uuid.UUID,
	) error
}

// Periodic job
func NewContinuousScreeningUpdateDatasetJob(interval time.Duration) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.ContinuousScreeningScanDatasetUpdatesArgs{}, &river.InsertOpts{
				Queue: models.CONTINUOUS_SCREENING_DATASET_UPDATE_QUEUE_NAME,
				UniqueOpts: river.UniqueOpts{
					ByQueue:  true,
					ByPeriod: interval,
				},
			}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

type ScanDatasetUpdatesWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningScanDatasetUpdatesArgs]
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	repo              scanDatasetUpdatesWorkerRepository
	screeningProvider scanDatasetUpdatesWorkerScreeningProvider
	blobRepo          repositories.BlobRepository
	taskEnqueuer      scanDatasetUpdatesWorkerTaskEnqueuer

	bucketUrl string
}

func NewScanDatasetUpdatesWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repo scanDatasetUpdatesWorkerRepository,
	screeningProvider scanDatasetUpdatesWorkerScreeningProvider,
	blobRepo repositories.BlobRepository,
	taskEnqueuer scanDatasetUpdatesWorkerTaskEnqueuer,
	bucketUrl string,
) *ScanDatasetUpdatesWorker {
	return &ScanDatasetUpdatesWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repo:               repo,
		screeningProvider:  screeningProvider,
		blobRepo:           blobRepo,
		taskEnqueuer:       taskEnqueuer,
		bucketUrl:          bucketUrl,
	}
}

func (w *ScanDatasetUpdatesWorker) Timeout(job *river.Job[models.ContinuousScreeningScanDatasetUpdatesArgs]) time.Duration {
	return 10 * time.Minute
}

func (w *ScanDatasetUpdatesWorker) Work(
	ctx context.Context,
	job *river.Job[models.ContinuousScreeningScanDatasetUpdatesArgs],
) error {
	exec := w.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Start dataset update")

	if w.bucketUrl == "" {
		logger.DebugContext(ctx, "No bucket url provided for storing delta files, skip processing")
		return nil
	}

	activeConfigs, err := w.repo.ListContinuousScreeningConfigs(ctx, exec)
	if err != nil {
		return err
	}
	if len(activeConfigs) == 0 {
		logger.DebugContext(ctx, "No active continuous screening configs found, skip processing")
		return nil
	}

	// Get datasets from screening provider and get only outdated datasets
	catalogs, err := w.screeningProvider.GetRawCatalog(ctx)
	if err != nil {
		return err
	}
	loadedDatasets := getLoadedDataset(ctx, catalogs)
	logger.DebugContext(ctx, "loaded dataset result", "datasets", loadedDatasets)
	datasetsWithVersionRange, err := w.getOutdatedDatasets(ctx, exec, loadedDatasets)
	if err != nil {
		return err
	}
	if len(datasetsWithVersionRange) == 0 {
		logger.DebugContext(ctx, "No outdated datasets found, skip processing")
		return nil
	}

	var blobInfos []deltaBlobInfo
	// Doesn't need to parallelize here. Much of case, we will have one catalog scope which groups datasets
	// datasetsWithVersionRange will be small (1 scope)
	// Inside a scope, we will have several versions to process
	for _, datasetWithRange := range datasetsWithVersionRange {
		resultBlobInfos, err := w.processDatasetVersion(
			ctx,
			datasetWithRange,
		)
		if err != nil {
			return err
		}
		blobInfos = append(blobInfos, resultBlobInfos...)
	}

	// Since the datasets are processed, we can update the last processed version in DB and enqueue task for each org
	err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		for _, blobInfo := range blobInfos {
			datasetUpdate, err := w.repo.CreateContinuousScreeningDatasetUpdate(ctx, tx, models.CreateContinuousScreeningDatasetUpdate{
				DatasetName:   blobInfo.datasetName,
				Version:       blobInfo.version,
				DeltaFilePath: blobInfo.blobKey,
				TotalItems:    blobInfo.lines,
			})
			if err != nil {
				return err
			}
			for _, config := range activeConfigs {
				update, err := w.repo.CreateContinuousScreeningUpdateJob(ctx, tx, models.CreateContinuousScreeningUpdateJob{
					DatasetUpdateId: datasetUpdate.Id,
					ConfigId:        config.Id,
					OrgId:           config.OrgId,
				})
				if err != nil {
					return err
				}
				err = w.taskEnqueuer.EnqueueContinuousScreeningApplyDeltaFileTask(
					ctx,
					tx,
					config.OrgId,
					update.Id,
				)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	logger.DebugContext(ctx, "Successfully processed dataset updates")
	return nil
}

type datasetWithVersionRange struct {
	dataset     models.OpenSanctionsRawDataset
	fromVersion string
	toVersion   string
}

// From datasets list, check if we know the dataset and save the last seen version in DB
func (w *ScanDatasetUpdatesWorker) getOutdatedDatasets(
	ctx context.Context,
	exec repositories.Executor,
	datasets []models.OpenSanctionsRawDataset,
) ([]datasetWithVersionRange, error) {
	logger := utils.LoggerFromContext(ctx)
	datasetsWithVersionRange := make([]datasetWithVersionRange, 0, len(datasets))

	// Check for each dataset if there is any update known in DB
	for _, dataset := range datasets {
		logger.DebugContext(ctx, "Checking dataset version", "dataset", dataset.Name, "version", dataset.Version)
		lastDatasetUpdate, err := w.repo.GetLastProcessedVersion(ctx, exec, dataset.Name)
		if err != nil {
			// Cold start for this dataset, save the version as the first one for the next run
			if errors.Is(err, models.NotFoundError) {
				logger.DebugContext(
					ctx,
					"No previous version found in DB, saving the current version",
					"dataset", dataset.Name,
					"version", dataset.Version,
				)
				_, err = w.repo.CreateContinuousScreeningDatasetUpdate(ctx, exec,
					models.CreateContinuousScreeningDatasetUpdate{
						DatasetName: dataset.Name,
						Version:     dataset.Version,
					})
				if err != nil {
					return nil, err
				}
				continue
			}
			return nil, err
		}
		// Compare the lastVersion with the current version of dataset from catalog
		if lastDatasetUpdate.Version == dataset.Version {
			// No update
			logger.DebugContext(ctx, "Same version as saved in DB, do nothing")
			continue
		} else if lastDatasetUpdate.Version > dataset.Version {
			// This should not happen, log a warning and skip
			logger.WarnContext(
				ctx,
				"Dataset version is older than the last processed version, skip it",
				"dataset", dataset.Name,
				"last_version", lastDatasetUpdate.Version,
				"current_version", dataset.Version,
			)
			continue
		}
		if dataset.DeltaUrl == nil {
			// No delta url, skip processing
			logger.DebugContext(ctx, "No delta url for dataset, skip processing", "dataset", dataset.Name)
			continue
		}

		datasetsWithVersionRange = append(
			datasetsWithVersionRange,
			datasetWithVersionRange{
				dataset:     dataset,
				fromVersion: lastDatasetUpdate.Version,
				toVersion:   dataset.Version,
			},
		)
	}

	return datasetsWithVersionRange, nil
}

// Helper to get detailed datasets info from catalog
// Filter out marble datasets
func getLoadedDataset(ctx context.Context, catalog models.OpenSanctionsRawCatalog) []models.OpenSanctionsRawDataset {
	logger := utils.LoggerFromContext(ctx)
	loadedDataset := append(catalog.Current, catalog.Outdated...)
	datasetList := make([]models.OpenSanctionsRawDataset, 0, len(loadedDataset))
	for _, dataset := range loadedDataset {
		d, ok := catalog.Datasets[dataset]
		if !ok {
			logger.WarnContext(ctx, "Loaded dataset is not present in catalog dataset, ignore it", "dataset", dataset)
			continue
		}
		if slices.Contains(d.Tags, MarbleContinuousScreeningTag) {
			// Skip marble datasets
			continue
		}

		datasetList = append(datasetList, d)
	}

	return datasetList
}

func (w *ScanDatasetUpdatesWorker) processDatasetVersion(
	ctx context.Context,
	d datasetWithVersionRange,
) ([]deltaBlobInfo, error) {
	logger := utils.LoggerFromContext(ctx)

	versions, err := w.getDeltaList(ctx, d)
	if err != nil {
		return nil, err
	}
	logger.DebugContext(ctx, "Fetched delta list", "delta_versions", versions)

	filteredVersions := slices.DeleteFunc(versions, func(record deltaRecord) bool {
		// Keep versions within (fromVersion, toVersion]
		if record.version <= d.fromVersion {
			return true
		}
		if record.version > d.toVersion {
			return true
		}
		return false
	})

	// Channel to collect results from concurrent downloads
	// Use buffered channel to avoid goroutines blocking
	blobInfoResultsChan := make(chan deltaBlobInfo, len(filteredVersions))

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(MaxConcurrentDatasetUpdates)

	for _, version := range filteredVersions {
		group.Go(func() error {
			logger.DebugContext(ctx, "Processing delta file for dataset update",
				"dataset", d.dataset.Name,
				"version", version.version,
				"url", version.url,
			)
			blobInfo, err := w.downloadDeltaFileAndSaveInBlob(ctx, version)
			if err != nil {
				logger.WarnContext(
					ctx,
					"Failed to download delta file and save in blob",
					"dataset", d.dataset.Name,
					"version", version.version,
					"error", err,
				)
				return err
			}
			blobInfoResultsChan <- blobInfo
			logger.DebugContext(ctx, "Finished processing delta file for dataset update",
				"dataset", d.dataset.Name,
				"version", version.version,
			)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		close(blobInfoResultsChan)
		return nil, err
	}
	close(blobInfoResultsChan)

	// Convert channel results to slice
	blobInfos := make([]deltaBlobInfo, 0, len(filteredVersions))
	for result := range blobInfoResultsChan {
		blobInfos = append(blobInfos, result)
	}

	return blobInfos, nil
}

type newlineCountingWriter struct {
	writer io.Writer
	count  int
}

func (w *newlineCountingWriter) Write(p []byte) (n int, err error) {
	n, err = w.writer.Write(p)
	if err != nil {
		return n, err
	}
	w.count += bytes.Count(p[:n], []byte("\n"))
	return n, nil
}

type deltaBlobInfo struct {
	lines       int
	datasetName string
	version     string
	blobKey     string
}

func (w *ScanDatasetUpdatesWorker) downloadDeltaFileAndSaveInBlob(ctx context.Context, version deltaRecord) (deltaBlobInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", version.url, nil)
	if err != nil {
		return deltaBlobInfo{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return deltaBlobInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return deltaBlobInfo{}, errors.Newf("failed to download delta file, status code: %d", resp.StatusCode)
	}

	key := fmt.Sprintf("%s/%s/%s.ndjson", ProviderUpdatesFolderName, version.datasetName, version.version)
	writer, err := w.blobRepo.OpenStream(
		ctx,
		w.bucketUrl,
		key,
		key,
	)
	if err != nil {
		return deltaBlobInfo{}, err
	}
	defer writer.Close()

	nlWriter := &newlineCountingWriter{writer: writer}

	_, err = io.Copy(nlWriter, resp.Body)
	if err != nil {
		return deltaBlobInfo{}, err
	}

	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Downloaded and saved delta file to blob",
		"dataset", version.datasetName,
		"version", version.version,
		"lines", nlWriter.count,
	)
	return deltaBlobInfo{
		lines:       nlWriter.count,
		datasetName: version.datasetName,
		version:     version.version,
		blobKey:     key,
	}, nil
}

type deltaRecord struct {
	datasetName string
	version     string
	url         string
}

func (w *ScanDatasetUpdatesWorker) getDeltaList(ctx context.Context,
	datasetWithRange datasetWithVersionRange,
) ([]deltaRecord, error) {
	if datasetWithRange.dataset.DeltaUrl == nil {
		// Should not happen
		return nil, errors.New("delta url is nil")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", *datasetWithRange.dataset.DeltaUrl, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Newf("failed to download delta file, status code: %d", resp.StatusCode)
	}
	var deltaList httpmodels.HTTPOpenSanctionDeltaList
	if err := json.NewDecoder(resp.Body).Decode(&deltaList); err != nil {
		return nil, err
	}

	records := make([]deltaRecord, 0, len(deltaList.Versions))
	for version, url := range deltaList.Versions {
		records = append(records, deltaRecord{
			datasetName: datasetWithRange.dataset.Name,
			version:     version,
			url:         url,
		})
	}
	return records, nil
}
