package continuous_screening

import (
	"bytes"
	"context"
	"encoding/json"
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
)

type datasetUpdateWorkerRepository interface {
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
}

type datasetUpdateWorkerCSUsecase interface {
	GetDataModelTableAndMapping(ctx context.Context, exec repositories.Executor,
		config models.ContinuousScreeningConfig, objectType string,
	) (models.Table, models.ContinuousScreeningDataModelMapping, error)
	GetIngestedObject(ctx context.Context, clientDbExec repositories.Executor, table models.Table,
		objectId string,
	) (models.DataModelObject, uuid.UUID, error)
	DoScreening(
		ctx context.Context,
		exec repositories.Executor,
		ingestedObject models.DataModelObject,
		mapping models.ContinuousScreeningDataModelMapping,
		config models.ContinuousScreeningConfig,
		objectType string,
		objectId string,
	) (models.ScreeningWithMatches, error)
	HandleCaseCreation(
		ctx context.Context,
		tx repositories.Transaction,
		config models.ContinuousScreeningConfig,
		objectId string,
		continuousScreeningWithMatches models.ContinuousScreeningWithMatches,
	) (models.Case, error)
}

type datasetUpdateWorkerScreeningProvider interface {
	GetRawCatalog(ctx context.Context) (models.OpenSanctionsRawCatalog, error)
}

type datasetUpdateWorkerBlobRepository interface {
	DeleteFile(ctx context.Context, bucketUrl, key string) error
	OpenStream(ctx context.Context, bucketUrl, key string, fileName string) (io.WriteCloser, error)
}

// Periodic job
func NewContinuousScreeningUpdateDatasetJob(interval time.Duration) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.ContinuousScreeningUpdateDatasetArgs{}, &river.InsertOpts{
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

type DatasetUpdateWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningUpdateDatasetArgs]
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	repo              datasetUpdateWorkerRepository
	screeningProvider datasetUpdateWorkerScreeningProvider
	blobRepo          datasetUpdateWorkerBlobRepository

	bucketUrl string
}

func NewDatasetUpdateWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repo datasetUpdateWorkerRepository,
	screeningProvider datasetUpdateWorkerScreeningProvider,
	blobRepo datasetUpdateWorkerBlobRepository,
	buckerUrl string,
) *DatasetUpdateWorker {
	return &DatasetUpdateWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repo:               repo,
		screeningProvider:  screeningProvider,
		blobRepo:           blobRepo,
		bucketUrl:          buckerUrl,
	}
}

func (w *DatasetUpdateWorker) Timeout(job *river.Job[models.ContinuousScreeningUpdateDatasetArgs]) time.Duration {
	// TODO: Should we change the timeout duration?
	return 10 * time.Minute
}

func (w *DatasetUpdateWorker) Work(ctx context.Context, job *river.Job[models.ContinuousScreeningUpdateDatasetArgs]) error {
	exec := w.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Start dataset update")

	catalogs, err := w.screeningProvider.GetRawCatalog(ctx)
	if err != nil {
		return err
	}

	loadedDatasets := getLoadedDataset(ctx, catalogs)
	logger.DebugContext(ctx, "loaded dataset result", "datasets", loadedDatasets)

	datasetsWithVersionRange := make([]datasetWithDeltaRange, 0, len(loadedDatasets))

	// Check if there is any update in DB
	for _, dataset := range loadedDatasets {
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
					return err
				}
				continue
			}
			return err
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
			datasetWithDeltaRange{
				dataset:     dataset,
				fromVersion: lastDatasetUpdate.Version,
				toVersion:   dataset.Version,
			},
		)
	}

	for _, datasetWithRange := range datasetsWithVersionRange {
		err := w.processDatasetVersion(
			ctx,
			datasetWithRange,
		)
		if err != nil {
			return err
		}
	}

	logger.DebugContext(ctx, "Finished")
	return nil
}

type datasetWithDeltaRange struct {
	dataset     models.OpenSanctionsRawDataset
	fromVersion string
	toVersion   string
}

// Helper to get detailed datasets info from catalog
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
		datasetList = append(datasetList, d)
	}

	return datasetList
}

func (w *DatasetUpdateWorker) processDatasetVersion(
	ctx context.Context,
	d datasetWithDeltaRange,
) error {
	logger := utils.LoggerFromContext(ctx)

	versions, err := w.getDeltaList(ctx, d)
	if err != nil {
		return err
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

	for _, version := range filteredVersions {
		logger.DebugContext(ctx, "Processing delta file for dataset update",
			"dataset", d.dataset.Name,
			"version", version.version,
			"url", version.url,
		)
		err := w.downloadDeltaFileAndSaveInBlob(ctx, version)
		if err != nil {
			return err
		}
		logger.DebugContext(ctx, "Finished processing delta file for dataset update",
			"dataset", d.dataset.Name,
			"version", version.version,
		)
	}

	return nil
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

func (w *DatasetUpdateWorker) downloadDeltaFileAndSaveInBlob(ctx context.Context, version deltaRecord) error {
	req, err := http.NewRequestWithContext(ctx, "GET", version.url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Newf("failed to download delta file, status code: %d", resp.StatusCode)
	}

	key := version.datasetName + "/" + version.version + ".ndjson"
	writer, err := w.blobRepo.OpenStream(
		ctx,
		w.bucketUrl,
		key,
		key,
	)
	if err != nil {
		return err
	}
	defer writer.Close()

	nlWriter := &newlineCountingWriter{writer: writer}

	_, err = io.Copy(nlWriter, resp.Body)
	if err != nil {
		return err
	}

	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Downloaded and saved delta file to blob",
		"dataset", version.datasetName,
		"version", version.version,
		"lines", nlWriter.count,
	)
	return nil
}

type deltaRecord struct {
	datasetName string
	version     string
	url         string
}

func (w *DatasetUpdateWorker) getDeltaList(ctx context.Context,
	datasetWithRange datasetWithDeltaRange,
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
