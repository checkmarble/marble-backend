package continuous_screening

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/hashicorp/go-set/v2"
	"github.com/riverqueue/river"
	"gocloud.dev/blob"
	"gopkg.in/yaml.v3"
)

const (
	MaxDeltaTracksPerOrg   = 1000
	ManifestFileName       = "manifest.yml"
	DeltasIndexFileName    = "deltas.json"
	FullDatasetFolderName  = "full-dataset"
	DeltaDatasetFolderName = "delta-dataset"
	DatasetFileContentType = "application/x-ndjson"
)

// TODO: Manifest can contains the `delta_url` fields, but this field only support URL and not local path.
// Don't fill this field for now, until we know how to provide the delta file to the indexer.
type ManifestDataset struct {
	Name    string `yaml:"name"`    // org UUID
	Path    string `yaml:"path"`    // path to entities file
	Version string `yaml:"version"` // version string e.g. "20251230-001"
}

type Manifest struct {
	Catalogs []any             `yaml:"catalogs,omitempty"`
	Datasets []ManifestDataset `yaml:"datasets"`
}

func (m *Manifest) upsertDataset(orgId string, datasetFile models.ContinuousScreeningDatasetFile) {
	for i, ds := range m.Datasets {
		if ds.Name == orgId {
			m.Datasets[i].Path = datasetFile.FilePath
			m.Datasets[i].Version = datasetFile.Version
			return
		}
	}
	m.Datasets = append(m.Datasets, ManifestDataset{
		Name:    orgId,
		Path:    datasetFile.FilePath,
		Version: datasetFile.Version,
	})
}

// DeltasIndex is the structure for deltas.json file that tracks all delta file versions
type DeltasIndex struct {
	Versions map[string]string `json:"versions"`
}

func (d *DeltasIndex) addVersion(version string, filePath string) {
	if d.Versions == nil {
		d.Versions = make(map[string]string)
	}
	d.Versions[version] = filePath
}

// Periodic job
func NewContinuousScreeningCreateFullDatasetJob(interval time.Duration) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.ContinuousScreeningCreateFullDatasetArgs{}, &river.InsertOpts{
				Queue: models.CONTINUOUS_SCREENING_CREATE_FULL_DATASET_QUEUE_NAME,
				UniqueOpts: river.UniqueOpts{
					ByQueue:  true,
					ByPeriod: interval,
				},
			}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
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

	ListOrgsWithContinuousScreeningConfigs(
		ctx context.Context,
		exec repositories.Executor,
	) ([]uuid.UUID, error)

	GetContinuousScreeningLatestDatasetFileByOrgId(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		fileType models.ContinuousScreeningDatasetFileType,
	) (*models.ContinuousScreeningDatasetFile, error)

	GetDataModel(
		ctx context.Context,
		exec repositories.Executor,
		organizationID string,
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
}

func NewCreateFullDatasetWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repo createFullDatasetWorkerRepository,
	ingestedDataReader createFullDatasetWorkerIngestedDataReader,
	blobRepository repositories.BlobRepository,
	bucketUrl string,
) *CreateFullDatasetWorker {
	return &CreateFullDatasetWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repo:               repo,
		ingestedDataReader: ingestedDataReader,
		blobRepository:     blobRepository,
		bucketUrl:          bucketUrl,
	}
}

func (w *CreateFullDatasetWorker) Timeout(job *river.Job[models.ContinuousScreeningCreateFullDatasetArgs]) time.Duration {
	// TODO: need to monitor the time it takes to create the full dataset
	return 1 * time.Hour
}

func (w *CreateFullDatasetWorker) Work(ctx context.Context,
	job *river.Job[models.ContinuousScreeningCreateFullDatasetArgs],
) error {
	exec := w.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Creating full dataset", "job", job)

	orgIdsWithConfigs, err := w.repo.ListOrgsWithContinuousScreeningConfigs(ctx, exec)
	if err != nil {
		return errors.Wrap(err, "failed to list orgs with continuous screening configs")
	}

	// Map to store dataset files from handleFirstFullDataset for manifest update
	orgDatasetFiles := make(map[uuid.UUID]models.ContinuousScreeningDatasetFile)

	for _, orgId := range orgIdsWithConfigs {
		// Check if the dataset file for this org exists
		datasetFile, err := w.repo.GetContinuousScreeningLatestDatasetFileByOrgId(ctx, exec,
			orgId, models.ContinuousScreeningDatasetFileTypeFull)
		if err != nil {
			return errors.Wrap(err, "failed to get dataset file by org id")
		}

		if datasetFile == nil {
			logger.DebugContext(ctx, "No dataset file found for org, creating new one", "orgId", orgId)
			newDatasetFile, err := w.handleFirstFullDataset(ctx, exec, orgId)
			if err != nil {
				return errors.Wrap(err, "failed to handle first full dataset")
			}
			orgDatasetFiles[orgId] = newDatasetFile
		} else {
			logger.DebugContext(ctx, "Dataset file found for org, patching it and creating new version",
				"orgId", orgId, "datasetFile", datasetFile)
			newDatasetFile, err := w.handlePatchDataset(ctx, exec, orgId, *datasetFile)
			if err != nil {
				return errors.Wrap(err, "failed to handle patch dataset")
			}
			orgDatasetFiles[orgId] = newDatasetFile
		}
	}

	// Update manifest with all org dataset files
	if len(orgDatasetFiles) > 0 {
		if err := w.updateManifest(ctx, orgDatasetFiles); err != nil {
			logger.ErrorContext(ctx, "Failed to update manifest", "error", err)
			// Don't return error to avoid job retries because the retry will not update the manifest
			return nil
		}
	}

	logger.DebugContext(ctx, "Successfully created full dataset")
	return nil
}

func (w *CreateFullDatasetWorker) handleFirstFullDataset(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID,
) (models.ContinuousScreeningDatasetFile, error) {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Creating first full dataset", "orgId", orgId)

	clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, orgId.String())
	if err != nil {
		return models.ContinuousScreeningDatasetFile{},
			errors.Wrap(err, "failed to get client db executor")
	}

	dataModel, err := w.repo.GetDataModel(ctx, exec, orgId.String(), false, false)
	if err != nil {
		return models.ContinuousScreeningDatasetFile{},
			errors.Wrap(err, "failed to get data model")
	}

	now := time.Now()
	cursorEntityId := ""

	version := generateNextVersion("", now)
	fileName := fmt.Sprintf("%s-entities.ftm.json", version)
	fullDatasetFileName := fmt.Sprintf("%s/%s/%s", orgId.String(), FullDatasetFolderName, fileName)

	blobWriter, err := w.blobRepository.OpenStreamWithOptions(ctx, w.bucketUrl, fullDatasetFileName,
		&blob.WriterOptions{
			ContentType: DatasetFileContentType,
		})
	if err != nil {
		return models.ContinuousScreeningDatasetFile{},
			errors.Wrap(err, "failed to open stream")
	}
	defer blobWriter.Close()

	for {
		deltaTracks, err := w.repo.ListContinuousScreeningLastChangeByEntityIds(
			ctx,
			exec,
			orgId,
			MaxDeltaTracksPerOrg,
			now,
			cursorEntityId,
		)
		if err != nil {
			return models.ContinuousScreeningDatasetFile{},
				errors.Wrap(err, "failed to list continuous screening last change by entity ids")
		}

		if len(deltaTracks) == 0 {
			break
		}

		typesAndObjectInternalIds := make(map[string]*set.Set[uuid.UUID])
		ingestedObjectsByType := make(map[string]map[uuid.UUID]models.DataModelObject)

		for _, deltaTrack := range deltaTracks {
			if deltaTrack.Operation == models.DeltaTrackOperationDelete {
				// Ignore deleted objects for the first full dataset creation
				continue
			}
			// Should always have an object internal id if the operation is not delete
			if _, ok := typesAndObjectInternalIds[deltaTrack.ObjectType]; !ok {
				typesAndObjectInternalIds[deltaTrack.ObjectType] = set.New[uuid.UUID](0)
			}
			typesAndObjectInternalIds[deltaTrack.ObjectType].Insert(*deltaTrack.ObjectInternalId)
		}

		for objectType, objectInternalIdsSet := range typesAndObjectInternalIds {
			objectInternalIds := objectInternalIdsSet.Slice()
			dataModelTable, ok := dataModel.Tables[objectType]
			// Check datamodel is correctly configured for the use case
			if !ok {
				return models.ContinuousScreeningDatasetFile{}, errors.Wrapf(models.NotFoundError,
					"table %s not found in data model", objectType)
			}
			if err := checkDataModelTableAndFieldsConfiguration(dataModelTable); err != nil {
				return models.ContinuousScreeningDatasetFile{},
					errors.Wrap(err, "data model table is not correctly configured for the use case")
			}
			ingestedObjectsFromDb, err := w.ingestedDataReader.QueryIngestedObjectByInternalIds(
				ctx, clientDbExec, dataModelTable, objectInternalIds)
			if err != nil {
				return models.ContinuousScreeningDatasetFile{},
					errors.Wrap(err, "failed to query ingested objects by internal ids")
			}
			if len(ingestedObjectsFromDb) != len(objectInternalIds) {
				return models.ContinuousScreeningDatasetFile{}, errors.Wrapf(models.NotFoundError,
					"number of ingested objects by internal ids %d does not match the number of object internal ids %d",
					len(ingestedObjectsFromDb), len(objectInternalIds))
			}

			ingestedObjects := make(map[uuid.UUID]models.DataModelObject, len(ingestedObjectsFromDb))
			for _, ingestedObject := range ingestedObjectsFromDb {
				id := toUUID(ingestedObject.Metadata["id"])
				ingestedObjects[id] = ingestedObject
			}
			ingestedObjectsByType[objectType] = ingestedObjects
		}

		// Delta tracks are sorted by entity id, so we can iterate over them and build the full dataset
		for _, deltaTrack := range deltaTracks {
			if deltaTrack.Operation == models.DeltaTrackOperationDelete {
				// Ignore deleted objects for the first full dataset creation
				continue
			}
			ingestedObjectData, ok := ingestedObjectsByType[deltaTrack.ObjectType][*deltaTrack.ObjectInternalId]
			if !ok {
				return models.ContinuousScreeningDatasetFile{}, errors.Wrapf(models.NotFoundError,
					"ingested object not found for object type %s and object internal id %s",
					deltaTrack.ObjectType, deltaTrack.ObjectInternalId)
			}
			datasetEntity := buildDatasetEntity(
				dataModel.Tables[deltaTrack.ObjectType],
				deltaTrack,
				ingestedObjectData,
			)
			entityJson, err := json.Marshal(datasetEntity)
			if err != nil {
				return models.ContinuousScreeningDatasetFile{},
					errors.Wrap(err, "failed to marshal dataset entity")
			}
			_, err = blobWriter.Write(append(entityJson, '\n'))
			if err != nil {
				return models.ContinuousScreeningDatasetFile{},
					errors.Wrap(err, "failed to write dataset entity to blob")
			}
		}

		cursorEntityId = deltaTracks[len(deltaTracks)-1].EntityId
	}

	// Create dataset file record and update delta tracks in a transaction
	// to ensure consistency
	var createdDatasetFile models.ContinuousScreeningDatasetFile
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

		createdDatasetFile = datasetFile
		return nil
	})
	if err != nil {
		return models.ContinuousScreeningDatasetFile{}, err
	}

	logger.DebugContext(ctx, "Successfully created first full dataset",
		"orgId", orgId, "filePath", fullDatasetFileName)

	return createdDatasetFile, nil
}

// handlePatchDataset handles patching an existing dataset by merging the previous dataset file
// with new delta tracks. Both sources are sorted by entity_id, enabling an efficient merge.
func (w *CreateFullDatasetWorker) handlePatchDataset(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID, previousDatasetFile models.ContinuousScreeningDatasetFile,
) (models.ContinuousScreeningDatasetFile, error) {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Patching dataset", "orgId", orgId, "previousVersion", previousDatasetFile.Version)

	clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, orgId.String())
	if err != nil {
		return models.ContinuousScreeningDatasetFile{},
			errors.Wrap(err, "failed to get client db executor")
	}

	dataModel, err := w.repo.GetDataModel(ctx, exec, orgId.String(), false, false)
	if err != nil {
		return models.ContinuousScreeningDatasetFile{},
			errors.Wrap(err, "failed to get data model")
	}

	now := time.Now()
	version := generateNextVersion(previousDatasetFile.Version, now)
	fileName := fmt.Sprintf("%s-entities.ftm.json", version)
	fullDatasetFileName := fmt.Sprintf("%s/%s/%s", orgId.String(), FullDatasetFolderName, fileName)
	deltaFileName := fmt.Sprintf("%s-delta.ftm.json", version)
	deltaDatasetFileName := fmt.Sprintf("%s/%s/%s", orgId.String(), DeltaDatasetFolderName, deltaFileName)

	// Open previous dataset file for reading
	previousBlob, err := w.blobRepository.GetBlob(ctx, w.bucketUrl, previousDatasetFile.FilePath)
	if err != nil {
		return models.ContinuousScreeningDatasetFile{},
			errors.Wrap(err, "failed to get previous dataset blob")
	}
	defer previousBlob.ReadCloser.Close()

	// Open new dataset file for writing
	newBlobWriter, err := w.blobRepository.OpenStreamWithOptions(ctx, w.bucketUrl, fullDatasetFileName,
		&blob.WriterOptions{
			ContentType: DatasetFileContentType,
		})
	if err != nil {
		return models.ContinuousScreeningDatasetFile{},
			errors.Wrap(err, "failed to open stream for new dataset")
	}
	defer newBlobWriter.Close()

	// Open delta file for writing
	deltaBlobWriter, err := w.blobRepository.OpenStreamWithOptions(ctx, w.bucketUrl, deltaDatasetFileName,
		&blob.WriterOptions{
			ContentType: DatasetFileContentType,
		})
	if err != nil {
		return models.ContinuousScreeningDatasetFile{},
			errors.Wrap(err, "failed to open stream for delta dataset")
	}
	defer deltaBlobWriter.Close()

	// Initialize the track batch state
	trackBatch := &trackBatchState{
		tracks:                nil,
		tracksByEntityId:      make(map[string]models.ContinuousScreeningDeltaTrack),
		ingestedObjectsByType: make(map[string]map[uuid.UUID]models.DataModelObject),
		currentIndex:          0,
		cursorEntityId:        "",
		exhausted:             false,
	}

	// Load first batch of tracks
	if err := w.loadNextTrackBatch(ctx, exec, clientDbExec, orgId, now, dataModel, trackBatch); err != nil {
		return models.ContinuousScreeningDatasetFile{},
			errors.Wrap(err, "failed to load first track batch")
	}

	// Read and merge old file with tracks using JSON decoder
	decoder := json.NewDecoder(previousBlob.ReadCloser)

	for {
		var oldEntity datasetEntity
		if err := decoder.Decode(&oldEntity); err != nil {
			if err == io.EOF {
				break
			}
			return models.ContinuousScreeningDatasetFile{},
				errors.Wrap(err, "failed to decode old entity")
		}

		// Process any tracks that come before this old entity (new ADDs)
		for !trackBatch.exhausted && trackBatch.currentIndex < len(trackBatch.tracks) {
			currentTrack := trackBatch.tracks[trackBatch.currentIndex]
			if currentTrack.EntityId >= oldEntity.Id {
				break
			}

			// This track's entity_id < old entity's id, so it's a new ADD
			if currentTrack.Operation != models.DeltaTrackOperationDelete {
				if err := w.writeTrackEntity(newBlobWriter, dataModel, trackBatch, currentTrack); err != nil {
					return models.ContinuousScreeningDatasetFile{},
						errors.Wrap(err, "failed to write new entity from track")
				}
				// Write ADD to delta file
				if err := writeDeltaEntityFromTrack(deltaBlobWriter, DeltaOperationAdd,
					dataModel, trackBatch, currentTrack); err != nil {
					return models.ContinuousScreeningDatasetFile{},
						errors.Wrap(err, "failed to write ADD delta entry")
				}
			}
			trackBatch.currentIndex++

			// Load next batch if current is exhausted
			if trackBatch.currentIndex >= len(trackBatch.tracks) {
				if err := w.loadNextTrackBatch(ctx, exec, clientDbExec, orgId, now, dataModel, trackBatch); err != nil {
					return models.ContinuousScreeningDatasetFile{},
						errors.Wrap(err, "failed to load next track batch")
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
					if err := writeDeltaDelete(deltaBlobWriter, currentTrack.EntityId); err != nil {
						return models.ContinuousScreeningDatasetFile{},
							errors.Wrap(err, "failed to write DEL delta entry")
					}
				case models.DeltaTrackOperationUpdate, models.DeltaTrackOperationAdd:
					// Write updated entity from track
					if err := w.writeTrackEntity(newBlobWriter, dataModel, trackBatch, currentTrack); err != nil {
						return models.ContinuousScreeningDatasetFile{},
							errors.Wrap(err, "failed to write updated entity from track")
					}
					// Write MOD to delta file (entity existed in old dataset)
					if err := writeDeltaEntityFromTrack(deltaBlobWriter, DeltaOperationMod,
						dataModel, trackBatch, currentTrack); err != nil {
						return models.ContinuousScreeningDatasetFile{},
							errors.Wrap(err, "failed to write MOD delta entry")
					}
				}
				trackBatch.currentIndex++

				// Load next batch if current is exhausted
				if trackBatch.currentIndex >= len(trackBatch.tracks) {
					if err := w.loadNextTrackBatch(ctx, exec, clientDbExec,
						orgId, now, dataModel, trackBatch); err != nil {
						return models.ContinuousScreeningDatasetFile{},
							errors.Wrap(err, "failed to load next track batch")
					}
				}
				continue
			}
		}

		// Old entity not affected, re-encode and write
		entityJson, err := json.Marshal(oldEntity)
		if err != nil {
			return models.ContinuousScreeningDatasetFile{},
				errors.Wrap(err, "failed to marshal old entity")
		}
		if _, err := newBlobWriter.Write(append(entityJson, '\n')); err != nil {
			return models.ContinuousScreeningDatasetFile{},
				errors.Wrap(err, "failed to write old entity to new blob")
		}
	}

	// Write any remaining tracks (new ADDs after the last old entity)
	for !trackBatch.exhausted {
		for trackBatch.currentIndex < len(trackBatch.tracks) {
			currentTrack := trackBatch.tracks[trackBatch.currentIndex]
			if currentTrack.Operation != models.DeltaTrackOperationDelete {
				if err := w.writeTrackEntity(newBlobWriter, dataModel, trackBatch, currentTrack); err != nil {
					return models.ContinuousScreeningDatasetFile{},
						errors.Wrap(err, "failed to write remaining entity from track")
				}
				// Write ADD to delta file
				if err := writeDeltaEntityFromTrack(deltaBlobWriter, DeltaOperationAdd,
					dataModel, trackBatch, currentTrack); err != nil {
					return models.ContinuousScreeningDatasetFile{},
						errors.Wrap(err, "failed to write ADD delta entry for remaining track")
				}
			}
			trackBatch.currentIndex++
		}

		if err := w.loadNextTrackBatch(ctx, exec, clientDbExec, orgId, now, dataModel, trackBatch); err != nil {
			return models.ContinuousScreeningDatasetFile{},
				errors.Wrap(err, "failed to load next track batch")
		}
	}

	// Create dataset file records and update delta tracks in a transaction
	var createdDatasetFile models.ContinuousScreeningDatasetFile
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

		createdDatasetFile = datasetFile
		return nil
	})
	if err != nil {
		return models.ContinuousScreeningDatasetFile{}, err
	}

	// Update the deltas.json index file with the new delta file
	if err := w.updateDeltasIndex(ctx, orgId, version, deltaDatasetFileName); err != nil {
		logger.ErrorContext(ctx, "Failed to update deltas index", "error", err, "orgId", orgId)
		// Don't return error to avoid job retries as the main dataset files are already created
	}

	logger.DebugContext(ctx, "Successfully patched dataset",
		"orgId", orgId, "previousVersion", previousDatasetFile.Version, "newVersion", version,
		"deltaFile", deltaDatasetFileName)

	return createdDatasetFile, nil
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
			id := toUUID(obj.Metadata["id"])
			ingestedObjects[id] = obj
		}
		state.ingestedObjectsByType[objectType] = ingestedObjects
	}

	state.tracks = tracks
	state.currentIndex = 0
	state.cursorEntityId = tracks[len(tracks)-1].EntityId
	return nil
}

// writeTrackEntity writes an entity from a track to the output blob
func (w *CreateFullDatasetWorker) writeTrackEntity(
	writer io.Writer,
	dataModel models.DataModel,
	state *trackBatchState,
	track models.ContinuousScreeningDeltaTrack,
) error {
	if track.ObjectInternalId == nil {
		return errors.Wrapf(models.NotFoundError,
			"track %s has no object internal id for non-delete operation", track.EntityId)
	}

	ingestedObjects, ok := state.ingestedObjectsByType[track.ObjectType]
	if !ok {
		return errors.Wrapf(models.NotFoundError,
			"no ingested objects for object type %s", track.ObjectType)
	}

	ingestedObjectData, ok := ingestedObjects[*track.ObjectInternalId]
	if !ok {
		return errors.Wrapf(models.NotFoundError,
			"ingested object not found for object type %s and internal id %s",
			track.ObjectType, track.ObjectInternalId)
	}

	entity := buildDatasetEntity(dataModel.Tables[track.ObjectType], track, ingestedObjectData)
	entityJson, err := json.Marshal(entity)
	if err != nil {
		return errors.Wrap(err, "failed to marshal entity")
	}

	if _, err := writer.Write(append(entityJson, '\n')); err != nil {
		return errors.Wrap(err, "failed to write entity")
	}

	return nil
}

// writeDeltaEntry writes a delta entry to the delta file
func writeDeltaEntry(writer io.Writer, op deltaOperation, entity any) error {
	entry := deltaEntry{
		Op:     op,
		Entity: entity,
	}
	entryJson, err := json.Marshal(entry)
	if err != nil {
		return errors.Wrap(err, "failed to marshal delta entry")
	}
	if _, err := writer.Write(append(entryJson, '\n')); err != nil {
		return errors.Wrap(err, "failed to write delta entry")
	}
	return nil
}

// writeDeltaEntityFromTrack writes a full entity delta entry (ADD or MOD)
func writeDeltaEntityFromTrack(
	writer io.Writer,
	op deltaOperation,
	dataModel models.DataModel,
	state *trackBatchState,
	track models.ContinuousScreeningDeltaTrack,
) error {
	if track.ObjectInternalId == nil {
		return errors.Wrapf(models.NotFoundError,
			"track %s has no object internal id for non-delete operation", track.EntityId)
	}

	ingestedObjects, ok := state.ingestedObjectsByType[track.ObjectType]
	if !ok {
		return errors.Wrapf(models.NotFoundError,
			"no ingested objects for object type %s", track.ObjectType)
	}

	ingestedObjectData, ok := ingestedObjects[*track.ObjectInternalId]
	if !ok {
		return errors.Wrapf(models.NotFoundError,
			"ingested object not found for object type %s and internal id %s",
			track.ObjectType, track.ObjectInternalId)
	}

	entity := buildDatasetEntity(dataModel.Tables[track.ObjectType], track, ingestedObjectData)
	return writeDeltaEntry(writer, op, entity)
}

// writeDeltaDelete writes a DEL delta entry with minimal entity (only id)
func writeDeltaDelete(writer io.Writer, entityId string) error {
	return writeDeltaEntry(writer, DeltaOperationDel, deltaEntityMinimal{Id: entityId})
}

func (w *CreateFullDatasetWorker) getOrCreateManifest(ctx context.Context) (Manifest, error) {
	logger := utils.LoggerFromContext(ctx)

	blob, err := w.blobRepository.GetBlob(ctx, w.bucketUrl, ManifestFileName)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			logger.DebugContext(ctx, "Manifest file not found, creating new one")
			return Manifest{Datasets: []ManifestDataset{}}, nil
		}
		return Manifest{}, errors.Wrap(err, "failed to get manifest blob")
	}
	defer blob.ReadCloser.Close()

	data, err := io.ReadAll(blob.ReadCloser)
	if err != nil {
		return Manifest{}, errors.Wrap(err, "failed to read manifest blob")
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, errors.Wrap(err, "failed to unmarshal manifest")
	}

	return manifest, nil
}

// Update or create manifest file with organization dataset information (version and path)
func (w *CreateFullDatasetWorker) updateManifest(
	ctx context.Context,
	orgDatasetFiles map[uuid.UUID]models.ContinuousScreeningDatasetFile,
) error {
	logger := utils.LoggerFromContext(ctx)

	manifest, err := w.getOrCreateManifest(ctx)
	if err != nil {
		return err
	}

	for orgId, datasetFile := range orgDatasetFiles {
		manifest.upsertDataset(orgId.String(), datasetFile)
	}

	manifestData, err := yaml.Marshal(&manifest)
	if err != nil {
		return errors.Wrap(err, "failed to marshal manifest")
	}

	writer, err := w.blobRepository.OpenStream(ctx, w.bucketUrl, ManifestFileName, ManifestFileName)
	if err != nil {
		return errors.Wrap(err, "failed to open stream for manifest")
	}
	defer writer.Close()

	if _, err := io.Copy(writer, bytes.NewReader(manifestData)); err != nil {
		return errors.Wrap(err, "failed to write manifest")
	}

	logger.DebugContext(ctx, "Successfully updated manifest",
		"datasetsCount", len(manifest.Datasets))

	return nil
}

// getOrCreateDeltasIndex reads the deltas.json file for an org or creates an empty one
func (w *CreateFullDatasetWorker) getOrCreateDeltasIndex(ctx context.Context, orgId uuid.UUID) (DeltasIndex, error) {
	logger := utils.LoggerFromContext(ctx)
	deltasIndexPath := fmt.Sprintf("%s/%s/%s", orgId.String(), DeltaDatasetFolderName, DeltasIndexFileName)

	blob, err := w.blobRepository.GetBlob(ctx, w.bucketUrl, deltasIndexPath)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			logger.DebugContext(ctx, "Deltas index file not found, creating new one", "orgId", orgId)
			return DeltasIndex{Versions: make(map[string]string)}, nil
		}
		return DeltasIndex{}, errors.Wrap(err, "failed to get deltas index blob")
	}
	defer blob.ReadCloser.Close()

	data, err := io.ReadAll(blob.ReadCloser)
	if err != nil {
		return DeltasIndex{}, errors.Wrap(err, "failed to read deltas index blob")
	}

	var deltasIndex DeltasIndex
	if err := json.Unmarshal(data, &deltasIndex); err != nil {
		return DeltasIndex{}, errors.Wrap(err, "failed to unmarshal deltas index")
	}

	if deltasIndex.Versions == nil {
		deltasIndex.Versions = make(map[string]string)
	}

	return deltasIndex, nil
}

// updateDeltasIndex updates the deltas.json file with a new delta file version
func (w *CreateFullDatasetWorker) updateDeltasIndex(
	ctx context.Context,
	orgId uuid.UUID,
	version string,
	deltaFilePath string,
) error {
	logger := utils.LoggerFromContext(ctx)

	deltasIndex, err := w.getOrCreateDeltasIndex(ctx, orgId)
	if err != nil {
		return err
	}

	deltasIndex.addVersion(version, deltaFilePath)

	deltasIndexData, err := json.Marshal(&deltasIndex)
	if err != nil {
		return errors.Wrap(err, "failed to marshal deltas index")
	}

	deltasIndexPath := fmt.Sprintf("%s/%s/%s", orgId.String(), DeltaDatasetFolderName, DeltasIndexFileName)
	writer, err := w.blobRepository.OpenStream(ctx, w.bucketUrl, deltasIndexPath, deltasIndexPath)
	if err != nil {
		return errors.Wrap(err, "failed to open stream for deltas index")
	}
	defer writer.Close()

	if _, err := io.Copy(writer, bytes.NewReader(deltasIndexData)); err != nil {
		return errors.Wrap(err, "failed to write deltas index")
	}

	logger.DebugContext(ctx, "Successfully updated deltas index",
		"orgId", orgId, "version", version, "versionsCount", len(deltasIndex.Versions))

	return nil
}

type datasetEntity struct {
	Id         string         `json:"id"`
	Schema     string         `json:"schema"`
	Properties map[string]any `json:"properties"`
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

func buildDatasetEntity(
	table models.Table,
	track models.ContinuousScreeningDeltaTrack,
	ingestedObjectData models.DataModelObject,
) datasetEntity {
	properties := make(map[string]any)

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
				strVal = fmt.Sprintf("%g", v)
			case bool:
				strVal = fmt.Sprintf("%t", v)
			default:
				strVal = fmt.Sprintf("%v", v)
			}

			if strVal != "" {
				propertyKey := field.FTMProperty.String()
				if existing, ok := properties[propertyKey]; ok {
					if list, ok := existing.([]string); ok {
						properties[propertyKey] = append(list, strVal)
					}
				} else {
					properties[propertyKey] = []string{strVal}
				}
			}
		}
	}

	return datasetEntity{
		Id:         track.EntityId,
		Schema:     table.FTMEntity.String(),
		Properties: properties,
	}
}

func toUUID(v any) uuid.UUID {
	switch val := v.(type) {
	case uuid.UUID:
		return val
	case [16]byte:
		return uuid.UUID(val)
	case []byte:
		id, _ := uuid.FromBytes(val)
		return id
	case string:
		id, _ := uuid.Parse(val)
		return id
	default:
		return uuid.Nil
	}
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
