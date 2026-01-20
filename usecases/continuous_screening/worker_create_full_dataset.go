package continuous_screening

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/worker_jobs"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/hashicorp/go-set/v2"
	"github.com/riverqueue/river"
	"gocloud.dev/blob"
)

const (
	MaxDeltaTracksPerOrg   = 1000
	FullDatasetFolderName  = "full-dataset"
	DeltaDatasetFolderName = "delta-dataset"
	DatasetFileContentType = "application/x-ndjson"
)

// Periodic job
func NewContinuousScreeningCreateFullDatasetPeriodicJob(orgId uuid.UUID, interval time.Duration) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.ContinuousScreeningCreateFullDatasetArgs{
					OrgId: orgId.String(),
				}, &river.InsertOpts{
					Queue: orgId.String(),
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: interval,
					},
				}
		},
		nil,
	)
}

type createFullDatasetWorkerRepository interface {
	ListContinuousScreeningLastChangeByEntityIds(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		limit uint64,
		toDate time.Time,
		cursorEntityId string,
	) ([]models.ContinuousScreeningDeltaTrack, error)

	GetContinuousScreeningConfigsByOrgId(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
	) ([]models.ContinuousScreeningConfig, error)

	GetContinuousScreeningLatestDatasetFileByOrgId(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		fileType models.ContinuousScreeningDatasetFileType,
	) (*models.ContinuousScreeningDatasetFile, error)

	GetDataModel(
		ctx context.Context,
		exec repositories.Executor,
		organizationID uuid.UUID,
		fetchEnumValues bool,
		useCache bool,
	) (models.DataModel, error)

	CreateContinuousScreeningDatasetFile(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreeningDatasetFile,
	) (models.ContinuousScreeningDatasetFile, error)

	UpdateDeltaTracksDatasetFileId(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		datasetFileId uuid.UUID,
		toDate time.Time,
	) error
}

type createFullDatasetWorkerIngestedDataReader interface {
	QueryIngestedObjectByInternalIds(
		ctx context.Context,
		exec repositories.Executor,
		table models.Table,
		internalObjectIds []uuid.UUID,
		metadataFields ...string,
	) ([]models.DataModelObject, error)
}

type CreateFullDatasetWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningCreateFullDatasetArgs]
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	repo               createFullDatasetWorkerRepository
	ingestedDataReader createFullDatasetWorkerIngestedDataReader
	blobRepository     repositories.BlobRepository
	bucketUrl          string

	jobInterval time.Duration
}

func NewCreateFullDatasetWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repo createFullDatasetWorkerRepository,
	ingestedDataReader createFullDatasetWorkerIngestedDataReader,
	blobRepository repositories.BlobRepository,
	bucketUrl string,
	jobInterval time.Duration,
) *CreateFullDatasetWorker {
	return &CreateFullDatasetWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repo:               repo,
		ingestedDataReader: ingestedDataReader,
		blobRepository:     blobRepository,
		bucketUrl:          bucketUrl,
		jobInterval:        jobInterval,
	}
}

func (w *CreateFullDatasetWorker) Timeout(job *river.Job[models.ContinuousScreeningCreateFullDatasetArgs]) time.Duration {
	// TODO: need to monitor the time it takes to create the full dataset
	return 1 * time.Hour
}

// For an org, create the full and delta dataset files.
// Check if there is a full dataset for this org, if not create it with handleFirstFullDataset.
// If there is a full dataset, patch it with handlePatchDataset and create a new delta dataset file.
// The full dataset is supposed to be sorted by entityID to be able to merge the previous dataset file with the new delta tracks.
func (w *CreateFullDatasetWorker) Work(ctx context.Context,
	job *river.Job[models.ContinuousScreeningCreateFullDatasetArgs],
) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Creating full dataset", "job", job)

	// TODO: fetch the interval from the org config
	if err := worker_jobs.AddStrideDelay(job, w.jobInterval); err != nil {
		return err
	}

	if w.bucketUrl == "" {
		logger.DebugContext(ctx, "No bucket url provided for storing full dataset, skipping", "job", job)
		return nil
	}

	orgId, err := uuid.Parse(job.Args.OrgId)
	if err != nil {
		return errors.Wrap(err, "failed to parse org id")
	}

	// Use a pinned connection to ensure the advisory lock is tied to this session
	exec, release, err := w.executorFactory.NewPinnedExecutor(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get pinned executor")
	}
	defer release()

	// Set a timeout for the session in case the worker hangs or is killed
	// This ensures Postgres will eventually release the lock even if the connection isn't closed properly
	timeout := w.Timeout(job)
	_, err = exec.Exec(ctx, fmt.Sprintf("SET idle_session_timeout = '%dms'", timeout.Milliseconds()))
	if err != nil {
		return errors.Wrap(err, "failed to set idle_session_timeout")
	}

	// Acquire an advisory lock to prevent concurrent jobs for the same org
	lockKey := fmt.Sprintf("create-full-dataset-%s", orgId.String())
	unlock, acquired, err := repositories.GetTryAdvisoryLock(ctx, exec, lockKey)
	if err != nil {
		return errors.Wrap(err, "failed to acquire advisory lock")
	}
	if !acquired {
		logger.DebugContext(ctx, "Another job is already creating full dataset for this org, skipping",
			"orgId", orgId)
		return nil
	}
	defer func() {
		if err := unlock(); err != nil {
			logger.ErrorContext(ctx, "failed to release advisory lock", "error", err)
		}
	}()

	// Check if the org has a continuous screening config
	configs, err := w.repo.GetContinuousScreeningConfigsByOrgId(ctx, exec, orgId)
	if err != nil {
		return errors.Wrap(err, "failed to get continuous screening configs by org id")
	}
	if len(configs) == 0 {
		logger.DebugContext(ctx, "No continuous screening config found for org, skipping", "orgId", orgId)
		return nil
	}

	// Check if the dataset file for this org exists
	datasetFile, err := w.repo.GetContinuousScreeningLatestDatasetFileByOrgId(ctx, exec,
		orgId, models.ContinuousScreeningDatasetFileTypeFull)
	if err != nil {
		return errors.Wrap(err, "failed to get dataset file by org id")
	}

	if datasetFile == nil {
		logger.DebugContext(ctx, "No dataset file found for org, creating new one", "orgId", orgId)
		err := w.handleFirstFullDataset(ctx, exec, orgId)
		if err != nil {
			return errors.Wrap(err, "failed to handle first full dataset")
		}
	} else {
		logger.DebugContext(ctx, "Dataset file found for org, patching it and creating new version",
			"orgId", orgId, "datasetFile", datasetFile)
		err := w.handlePatchDataset(ctx, exec, orgId, *datasetFile)
		if err != nil {
			return errors.Wrap(err, "failed to handle patch dataset")
		}
	}

	logger.DebugContext(ctx, "Successfully created full dataset")
	return nil
}

// Create the first full dataset for the org.
// The flow is simpler than the patch dataset and we don't need to create a delta dataset file.
func (w *CreateFullDatasetWorker) handleFirstFullDataset(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID,
) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Creating first full dataset", "orgId", orgId)

	clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return errors.Wrap(err, "failed to get client db executor")
	}

	dataModel, err := w.repo.GetDataModel(ctx, exec, orgId, false, false)
	if err != nil {
		return errors.Wrap(err, "failed to get data model")
	}

	now := time.Now()
	version := generateNextVersion("", now)

	fileName := fmt.Sprintf("%s-entities.ftm.json", version)
	fullDatasetFileName := fmt.Sprintf("%s/%s/%s/%s", OrgDatasetsFolderName, orgId.String(), FullDatasetFolderName, fileName)

	trackBatch := &trackBatchState{}

	// Load first batch of tracks to check if we have anything to do
	if err := w.loadNextTrackBatch(ctx, exec, clientDbExec, orgId, now, dataModel, trackBatch); err != nil {
		return errors.Wrap(err, "failed to load first track batch")
	}

	if trackBatch.exhausted {
		logger.DebugContext(ctx, "No delta tracks found for first full dataset, skipping", "orgId", orgId)
		return nil
	}

	blobWriter, err := w.blobRepository.OpenStreamWithOptions(ctx, w.bucketUrl, fullDatasetFileName,
		&blob.WriterOptions{
			ContentType: DatasetFileContentType,
		})
	if err != nil {
		return errors.Wrap(err, "failed to open stream")
	}
	defer blobWriter.Close()

	fullDatasetEncoder := json.NewEncoder(blobWriter)

	for !trackBatch.exhausted {
		for trackBatch.currentIndex < len(trackBatch.tracks) {
			currentTrack := trackBatch.tracks[trackBatch.currentIndex]
			if currentTrack.Operation == models.DeltaTrackOperationDelete {
				// Ignore deleted objects for the first full dataset creation
				trackBatch.currentIndex++
				continue
			}

			entity, err := w.getDatasetEntityFromTrack(dataModel,
				trackBatch.ingestedObjectsByType, currentTrack)
			if err != nil {
				return errors.Wrap(err, "failed to get dataset entity from track")
			}

			if err := writeDatasetEntity(fullDatasetEncoder, entity); err != nil {
				return errors.Wrap(err, "failed to write dataset entity to blob")
			}
			trackBatch.currentIndex++
		}

		if err := w.loadNextTrackBatch(ctx, exec, clientDbExec, orgId, now, dataModel, trackBatch); err != nil {
			return errors.Wrap(err, "failed to load next track batch")
		}
	}

	// Create dataset file record and update delta tracks in a transaction
	// to ensure consistency
	err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		// Create a new dataset file record
		datasetFile, err := w.repo.CreateContinuousScreeningDatasetFile(ctx, tx,
			models.CreateContinuousScreeningDatasetFile{
				OrgId:    orgId,
				FileType: models.ContinuousScreeningDatasetFileTypeFull,
				Version:  version,
				FilePath: fullDatasetFileName,
			})
		if err != nil {
			return errors.Wrap(err, "failed to create dataset file record")
		}

		// Update all delta tracks without dataset_file_id for this org to reference the new dataset file
		err = w.repo.UpdateDeltaTracksDatasetFileId(ctx, tx, orgId, datasetFile.Id, now)
		if err != nil {
			return errors.Wrap(err, "failed to update delta tracks dataset file id")
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to create dataset file record")
	}

	logger.DebugContext(ctx, "Successfully created first full dataset",
		"orgId", orgId, "filePath", fullDatasetFileName)

	return nil
}

// handlePatchDataset handles patching an existing dataset by merging the previous dataset file
// with new delta tracks. Both sources are sorted by entity_id, enabling an efficient merge.
func (w *CreateFullDatasetWorker) handlePatchDataset(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID, previousDatasetFile models.ContinuousScreeningDatasetFile,
) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Patching dataset", "orgId", orgId, "previousVersion", previousDatasetFile.Version)

	clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return errors.Wrap(err, "failed to get client db executor")
	}

	dataModel, err := w.repo.GetDataModel(ctx, exec, orgId, false, false)
	if err != nil {
		return errors.Wrap(err, "failed to get data model")
	}

	now := time.Now()
	version := generateNextVersion(previousDatasetFile.Version, now)

	fileName := fmt.Sprintf("%s-entities.ftm.json", version)
	fullDatasetFileName := fmt.Sprintf("%s/%s/%s/%s", OrgDatasetsFolderName, orgId.String(), FullDatasetFolderName, fileName)

	deltaFileName := fmt.Sprintf("%s-delta.ftm.json", version)
	deltaDatasetFileName := fmt.Sprintf("%s/%s/%s/%s", OrgDatasetsFolderName, orgId.String(), DeltaDatasetFolderName, deltaFileName)

	previousBlob, err := w.blobRepository.GetBlob(ctx, w.bucketUrl, previousDatasetFile.FilePath)
	if err != nil {
		return errors.Wrap(err, "failed to get previous dataset blob")
	}
	defer previousBlob.ReadCloser.Close()

	trackBatch := &trackBatchState{}

	// Load first batch of tracks
	if err := w.loadNextTrackBatch(ctx, exec, clientDbExec, orgId, now, dataModel, trackBatch); err != nil {
		return errors.Wrap(err, "failed to load first track batch")
	}

	if trackBatch.exhausted {
		logger.DebugContext(ctx, "No new tracks to process for dataset patch, skipping", "orgId", orgId)
		return nil
	}

	// Full dataset file
	newBlobWriter, err := w.blobRepository.OpenStreamWithOptions(ctx, w.bucketUrl, fullDatasetFileName,
		&blob.WriterOptions{
			ContentType: DatasetFileContentType,
		})
	if err != nil {
		return errors.Wrap(err, "failed to open stream for new dataset")
	}
	defer newBlobWriter.Close()

	fullDatasetEncoder := json.NewEncoder(newBlobWriter)

	// Delta dataset file
	deltaBlobWriter, err := w.blobRepository.OpenStreamWithOptions(ctx, w.bucketUrl, deltaDatasetFileName,
		&blob.WriterOptions{
			ContentType: DatasetFileContentType,
		})
	if err != nil {
		return errors.Wrap(err, "failed to open stream for delta dataset")
	}
	defer deltaBlobWriter.Close()

	deltaDatasetEncoder := json.NewEncoder(deltaBlobWriter)

	// Read and merge old file with tracks using JSON decoder
	decoder := json.NewDecoder(previousBlob.ReadCloser)

	// This loop reads the previous dataset file and merges it with the new tracks
	// Read an entity from the previous dataset file and process the tracks that come before it (new ADDs)
	// Then determine if the current track affects the old entity (update or delete)
	// If not, the entity is re-encoded and written to the new dataset file because not impacted by modifications from the tracks
	for {
		var oldEntity datasetEntity
		if err := decoder.Decode(&oldEntity); err != nil {
			if err == io.EOF {
				break
			}
			return errors.Wrap(err, "failed to decode old entity")
		}

		// Process any tracks that come before this old entity (new ADDs)
		for !trackBatch.exhausted && trackBatch.currentIndex < len(trackBatch.tracks) {
			currentTrack := trackBatch.tracks[trackBatch.currentIndex]
			if currentTrack.EntityId >= oldEntity.Id {
				break
			}

			// This track's entity_id < old entity's id, so it's a new ADD
			if currentTrack.Operation != models.DeltaTrackOperationDelete {
				entity, err := w.getDatasetEntityFromTrack(dataModel,
					trackBatch.ingestedObjectsByType, currentTrack)
				if err != nil {
					return errors.Wrap(err, "failed to get dataset entity from track")
				}

				if err := writeDatasetEntity(fullDatasetEncoder, entity); err != nil {
					return errors.Wrap(err, "failed to write new entity from track")
				}
				// Write ADD to delta file
				if err := writeDeltaEntry(deltaDatasetEncoder, DeltaOperationAdd, entity); err != nil {
					return errors.Wrap(err, "failed to write ADD delta entry")
				}
			}
			trackBatch.currentIndex++

			// Load next batch if current is exhausted
			if trackBatch.currentIndex >= len(trackBatch.tracks) {
				if err := w.loadNextTrackBatch(ctx, exec, clientDbExec, orgId, now, dataModel, trackBatch); err != nil {
					return errors.Wrap(err, "failed to load next track batch")
				}
			}
		}

		// Check if old entity is affected by a track
		if !trackBatch.exhausted && trackBatch.currentIndex < len(trackBatch.tracks) {
			currentTrack := trackBatch.tracks[trackBatch.currentIndex]
			if currentTrack.EntityId == oldEntity.Id {
				// Entity is affected by this track
				switch currentTrack.Operation {
				case models.DeltaTrackOperationDelete:
					// Skip writing this entity (delete it)
					// Write DEL to delta file
					if err := writeDeltaDelete(deltaDatasetEncoder, currentTrack.EntityId); err != nil {
						return errors.Wrap(err, "failed to write DEL delta entry")
					}
				case models.DeltaTrackOperationUpdate, models.DeltaTrackOperationAdd:
					entity, err := w.getDatasetEntityFromTrack(dataModel,
						trackBatch.ingestedObjectsByType, currentTrack)
					if err != nil {
						return errors.Wrap(err, "failed to get dataset entity from track")
					}

					// Write updated entity from track
					if err := writeDatasetEntity(fullDatasetEncoder, entity); err != nil {
						return errors.Wrap(err, "failed to write updated entity from track")
					}
					// Write MOD to delta file (entity existed in old dataset)
					if err := writeDeltaEntry(deltaDatasetEncoder,
						DeltaOperationMod, entity); err != nil {
						return errors.Wrap(err, "failed to write MOD delta entry")
					}
				}
				trackBatch.currentIndex++

				// Load next batch if current is exhausted
				if trackBatch.currentIndex >= len(trackBatch.tracks) {
					if err := w.loadNextTrackBatch(ctx, exec, clientDbExec,
						orgId, now, dataModel, trackBatch); err != nil {
						return errors.Wrap(err, "failed to load next track batch")
					}
				}
				continue
			}
		}

		// Old entity not affected, re-encode and write
		if err := fullDatasetEncoder.Encode(oldEntity); err != nil {
			return errors.Wrap(err, "failed to encode old entity to new blob")
		}
	}

	// Write any remaining tracks (new ADDs after the last old entity)
	for !trackBatch.exhausted {
		for trackBatch.currentIndex < len(trackBatch.tracks) {
			currentTrack := trackBatch.tracks[trackBatch.currentIndex]
			if currentTrack.Operation != models.DeltaTrackOperationDelete {
				entity, err := w.getDatasetEntityFromTrack(dataModel,
					trackBatch.ingestedObjectsByType, currentTrack)
				if err != nil {
					return errors.Wrap(err, "failed to get dataset entity from track")
				}

				if err := writeDatasetEntity(fullDatasetEncoder, entity); err != nil {
					return errors.Wrap(err, "failed to write remaining entity from track")
				}
				// Write ADD to delta file
				if err := writeDeltaEntry(deltaDatasetEncoder, DeltaOperationAdd, entity); err != nil {
					return errors.Wrap(err, "failed to write ADD delta entry for remaining track")
				}
			}
			trackBatch.currentIndex++
		}

		if err := w.loadNextTrackBatch(ctx, exec, clientDbExec, orgId, now, dataModel, trackBatch); err != nil {
			return errors.Wrap(err, "failed to load next track batch")
		}
	}

	// Create dataset file records and update delta tracks in a transaction
	err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		datasetFile, err := w.repo.CreateContinuousScreeningDatasetFile(ctx, tx,
			models.CreateContinuousScreeningDatasetFile{
				OrgId:    orgId,
				FileType: models.ContinuousScreeningDatasetFileTypeFull,
				Version:  version,
				FilePath: fullDatasetFileName,
			})
		if err != nil {
			return errors.Wrap(err, "failed to create full dataset file record")
		}

		// Create delta dataset file record
		_, err = w.repo.CreateContinuousScreeningDatasetFile(ctx, tx,
			models.CreateContinuousScreeningDatasetFile{
				OrgId:    orgId,
				FileType: models.ContinuousScreeningDatasetFileTypeDelta,
				Version:  version,
				FilePath: deltaDatasetFileName,
			})
		if err != nil {
			return errors.Wrap(err, "failed to create delta dataset file record")
		}

		err = w.repo.UpdateDeltaTracksDatasetFileId(ctx, tx, orgId, datasetFile.Id, now)
		if err != nil {
			return errors.Wrap(err, "failed to update delta tracks dataset file id")
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to create dataset file record")
	}

	logger.DebugContext(ctx, "Successfully patched dataset",
		"orgId", orgId, "previousVersion", previousDatasetFile.Version, "newVersion", version,
		"deltaFile", deltaDatasetFileName)

	return nil
}

// trackBatchState holds the current state of the track batch being processed
type trackBatchState struct {
	tracks                []models.ContinuousScreeningDeltaTrack
	tracksByEntityId      map[string]models.ContinuousScreeningDeltaTrack
	ingestedObjectsByType map[string]map[uuid.UUID]models.DataModelObject
	currentIndex          int
	cursorEntityId        string
	exhausted             bool
}

// loadNextTrackBatch loads the next batch of tracks and their ingested data
func (w *CreateFullDatasetWorker) loadNextTrackBatch(
	ctx context.Context,
	exec repositories.Executor,
	clientDbExec repositories.Executor,
	orgId uuid.UUID,
	toDate time.Time,
	dataModel models.DataModel,
	state *trackBatchState,
) error {
	if state.exhausted {
		return nil
	}

	tracks, err := w.repo.ListContinuousScreeningLastChangeByEntityIds(
		ctx, exec, orgId, MaxDeltaTracksPerOrg, toDate, state.cursorEntityId,
	)
	if err != nil {
		return errors.Wrap(err, "failed to list tracks")
	}

	if len(tracks) == 0 {
		state.exhausted = true
		state.tracks = nil
		state.tracksByEntityId = make(map[string]models.ContinuousScreeningDeltaTrack)
		state.ingestedObjectsByType = make(map[string]map[uuid.UUID]models.DataModelObject)
		state.currentIndex = 0
		return nil
	}

	// Build map of entity_id to track for quick lookup
	state.tracksByEntityId = make(map[string]models.ContinuousScreeningDeltaTrack, len(tracks))
	for _, track := range tracks {
		state.tracksByEntityId[track.EntityId] = track
	}

	// Group object internal IDs by type for batch fetching
	typesAndObjectInternalIds := make(map[string]*set.Set[uuid.UUID])
	for _, track := range tracks {
		if track.Operation == models.DeltaTrackOperationDelete {
			continue
		}
		if track.ObjectInternalId == nil {
			continue
		}
		if _, ok := typesAndObjectInternalIds[track.ObjectType]; !ok {
			typesAndObjectInternalIds[track.ObjectType] = set.New[uuid.UUID](0)
		}
		typesAndObjectInternalIds[track.ObjectType].Insert(*track.ObjectInternalId)
	}

	// Fetch ingested objects in batches by type
	state.ingestedObjectsByType = make(map[string]map[uuid.UUID]models.DataModelObject)
	for objectType, objectInternalIdsSet := range typesAndObjectInternalIds {
		objectInternalIds := objectInternalIdsSet.Slice()
		dataModelTable, ok := dataModel.Tables[objectType]
		if !ok {
			return errors.Wrapf(models.NotFoundError,
				"table %s not found in data model", objectType)
		}
		if err := checkDataModelTableAndFieldsConfiguration(dataModelTable); err != nil {
			return errors.Wrap(err, "data model table is not correctly configured")
		}

		ingestedObjectsFromDb, err := w.ingestedDataReader.QueryIngestedObjectByInternalIds(
			ctx, clientDbExec, dataModelTable, objectInternalIds)
		if err != nil {
			return errors.Wrap(err, "failed to query ingested objects")
		}
		if len(ingestedObjectsFromDb) != len(objectInternalIds) {
			return errors.Wrapf(models.NotFoundError,
				"ingested objects count %d does not match expected %d",
				len(ingestedObjectsFromDb), len(objectInternalIds))
		}

		ingestedObjects := make(map[uuid.UUID]models.DataModelObject, len(ingestedObjectsFromDb))
		for _, obj := range ingestedObjectsFromDb {
			id, err := getIngestedObjectInternalId(obj)
			if err != nil {
				return err
			}
			ingestedObjects[id] = obj
		}
		state.ingestedObjectsByType[objectType] = ingestedObjects
	}

	state.tracks = tracks
	state.currentIndex = 0
	state.cursorEntityId = tracks[len(tracks)-1].EntityId
	return nil
}

// getDatasetEntityFromTrack builds a datasetEntity from a track and its associated ingested data
// Use for building the dataset entities to write to the new dataset file and the delta file
func (w *CreateFullDatasetWorker) getDatasetEntityFromTrack(
	dataModel models.DataModel,
	ingestedObjectsByType map[string]map[uuid.UUID]models.DataModelObject,
	track models.ContinuousScreeningDeltaTrack,
) (datasetEntity, error) {
	if track.ObjectInternalId == nil {
		return datasetEntity{}, errors.Wrapf(models.NotFoundError,
			"track %s has no object internal id for non-delete operation", track.EntityId)
	}

	ingestedObjects, ok := ingestedObjectsByType[track.ObjectType]
	if !ok {
		return datasetEntity{}, errors.Wrapf(models.NotFoundError,
			"no ingested objects for object type %s", track.ObjectType)
	}

	ingestedObjectData, ok := ingestedObjects[*track.ObjectInternalId]
	if !ok {
		return datasetEntity{}, errors.Wrapf(models.NotFoundError,
			"ingested object not found for object type %s and internal id %s",
			track.ObjectType, track.ObjectInternalId)
	}

	return buildDatasetEntity(dataModel.Tables[track.ObjectType], track, ingestedObjectData)
}

// writeDatasetEntity writes a datasetEntity to the output blob in NDJSON format
func writeDatasetEntity(encoder *json.Encoder, entity datasetEntity) error {
	if err := encoder.Encode(entity); err != nil {
		return errors.Wrap(err, "failed to encode entity")
	}

	return nil
}

// writeDeltaEntry writes a delta entry to the delta file
func writeDeltaEntry(encoder *json.Encoder, op deltaOperation, entity any) error {
	entry := deltaEntry{
		Op:     op,
		Entity: entity,
	}
	if err := encoder.Encode(entry); err != nil {
		return errors.Wrap(err, "failed to encode delta entry")
	}
	return nil
}

// writeDeltaDelete writes a DEL delta entry with minimal entity (only id)
func writeDeltaDelete(encoder *json.Encoder, entityId string) error {
	return writeDeltaEntry(encoder, DeltaOperationDel, deltaEntityMinimal{Id: entityId})
}

type datasetEntity struct {
	Id         string              `json:"id"`
	Schema     string              `json:"schema"`
	Properties map[string][]string `json:"properties"`
}

// Delta file structures for OpenSanctions/Yente format
type deltaOperation string

const (
	DeltaOperationAdd deltaOperation = "ADD"
	DeltaOperationMod deltaOperation = "MOD"
	DeltaOperationDel deltaOperation = "DEL"
)

type deltaEntry struct {
	Op     deltaOperation `json:"op"`
	Entity any            `json:"entity"`
}

// deltaEntityMinimal is used for DEL operations (only id required)
type deltaEntityMinimal struct {
	Id string `json:"id"`
}

// From ingested data, build the dataset entity.
// Normalize the FTM properties values.
func buildDatasetEntity(
	table models.Table,
	track models.ContinuousScreeningDeltaTrack,
	ingestedObjectData models.DataModelObject,
) (datasetEntity, error) {
	properties := make(map[string][]string)

	// Sort field names for deterministic output
	fieldNames := make([]string, 0, len(table.Fields))
	for name := range table.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

	for _, fieldName := range fieldNames {
		field := table.Fields[fieldName]
		if field.FTMProperty != nil {
			val := ingestedObjectData.Data[field.Name]
			if val == nil {
				continue
			}

			var strVal string
			switch v := val.(type) {
			case string:
				strVal = v
			case time.Time:
				if field.DataType == models.Timestamp {
					strVal = v.Format(time.RFC3339)
				} else {
					strVal = v.Format("2006-01-02")
				}
			case int, int8, int16, int32, int64:
				strVal = fmt.Sprintf("%d", v)
			case uint, uint8, uint16, uint32, uint64:
				strVal = fmt.Sprintf("%d", v)
			case float32, float64:
				strVal = fmt.Sprintf("%f", v)
			case bool:
				strVal = fmt.Sprintf("%t", v)
			default:
				strVal = fmt.Sprintf("%v", v)
			}

			if strVal != "" {
				strVal = normalizeFTMPropertyValue(*field.FTMProperty, strVal)

				propertyKey := field.FTMProperty.String()
				properties[propertyKey] = append(properties[propertyKey], strVal)
			}
		}
	}

	// Add metadata in `notes` property
	metadata := models.EntityNoteMetadata{
		ObjectId:   track.ObjectId,
		ObjectType: track.ObjectType,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return datasetEntity{}, errors.Wrap(err, "failed to marshal entity metadata")
	}

	notesKey := models.FollowTheMoneyPropertyNotes.String()
	properties[notesKey] = append(properties[notesKey], string(metadataJSON))

	return datasetEntity{
		Id:         track.EntityId,
		Schema:     table.FTMEntity.String(),
		Properties: properties,
	}, nil
}

// generateNextVersion generates the next version string based on the previous version and current date.
// Version format: "YYYYMMDDHHMMSS-XXX" where XXX is a zero-padded index (e.g., "20251230143005-001").
// The version is string incrementable.
func generateNextVersion(previousVersion string, now time.Time) string {
	currentPrefix := now.Format("20060102150405")

	parts := strings.Split(previousVersion, "-")
	if len(parts) != 2 {
		return currentPrefix + "-001"
	}

	prevPrefix := parts[0]
	prevSuffix := parts[1]

	// If the current time results in a prefix that is less than or equal to the previous one,
	// we must increment the previous version to ensure it remains "string incrementable".
	if currentPrefix <= prevPrefix {
		index := 1
		if i, err := strconv.Atoi(prevSuffix); err == nil {
			index = i + 1
		}
		return fmt.Sprintf("%s-%03d", prevPrefix, index)
	}

	// Current time is newer than previous version's time prefix, start fresh with "001"
	return currentPrefix + "-001"
}

// normalizeFTMPropertyValue applies all necessary normalizations (country, date, etc.)
// to a FTM property value based on its type.
func normalizeFTMPropertyValue(property models.FollowTheMoneyProperty, value string) string {
	value = normalizeCountryFTMPropertyValue(property, value)
	value = normalizeDateFTMPropertyValue(property, value)
	return value
}

// countryFTMProperties contains FTM properties that should be normalized to lowercase 2-letter country codes
var countryFTMProperties = map[models.FollowTheMoneyProperty]bool{
	models.FollowTheMoneyPropertyCountry:      true,
	models.FollowTheMoneyPropertyNationality:  true,
	models.FollowTheMoneyPropertyBirthCountry: true,
	models.FollowTheMoneyPropertyCitizenship:  true,
	models.FollowTheMoneyPropertyJurisdiction: true,
}

// normalizeCountryFTMPropertyValue converts country-related FTM property values to lowercase 2-letter ISO codes.
// For non-country properties, the value is returned unchanged.
// If the country cannot be identified, returns the original value unchanged.
func normalizeCountryFTMPropertyValue(property models.FollowTheMoneyProperty, value string) string {
	if !countryFTMProperties[property] {
		return value
	}

	alpha2 := pure_utils.CountryToAlpha2(value)
	if alpha2 == "" {
		return value
	}

	return strings.ToLower(alpha2)
}

// dateFTMProperties contains FTM properties that should be normalized to YYYY-MM-DD format
var dateFTMProperties = map[models.FollowTheMoneyProperty]bool{
	models.FollowTheMoneyPropertyBirthDate: true,
	models.FollowTheMoneyPropertyDeathDate: true,
}

// Common date formats to try when parsing date strings
var dateFormats = []string{
	"2006-01-02",          // ISO 8601: YYYY-MM-DD
	"2006/01/02",          // YYYY/MM/DD
	"2006-01-02T15:04:05", // ISO 8601 with time
	time.RFC3339,          // RFC 3339
	"20060102",            // YYYYMMDD compact
}

// normalizeDateFTMPropertyValue converts date-related FTM property values to YYYY-MM-DD format.
// For non-date properties, the value is returned unchanged.
// If the date cannot be parsed, returns the original value unchanged.
func normalizeDateFTMPropertyValue(property models.FollowTheMoneyProperty, value string) string {
	if !dateFTMProperties[property] {
		return value
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}

	// Try to parse the date using various formats
	for _, format := range dateFormats {
		if t, err := time.Parse(format, value); err == nil {
			return t.Format("2006-01-02")
		}
	}

	// If no format matched, return the original value
	return value
}
