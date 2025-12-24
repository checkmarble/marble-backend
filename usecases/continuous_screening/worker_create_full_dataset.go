package continuous_screening

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/hashicorp/go-set/v2"
	"github.com/riverqueue/river"
)

const (
	MaxDeltaTracksPerOrg   = 1000
	ManifestFileName       = "manifest.json"
	DeltaFilesName         = "delta.json"
	FullDatasetFolderName  = "full-dataset"
	DeltaDatasetFolderName = "delta-dataset"
)

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
	for _, orgId := range orgIdsWithConfigs {
		// Check if the dataset file for this org exists
		datasetFile, err := w.repo.GetContinuousScreeningLatestDatasetFileByOrgId(ctx, exec,
			orgId, models.ContinuousScreeningDatasetFileTypeFull)
		if err != nil {
			return errors.Wrap(err, "failed to get dataset file by org id")
		}

		if datasetFile == nil {
			logger.DebugContext(ctx, "No dataset file found for org, creating new one", "orgId", orgId)
			err = w.handleFirstFullDataset(ctx, exec, orgId)
			if err != nil {
				return errors.Wrap(err, "failed to handle first full dataset")
			}
		} else {
			logger.DebugContext(ctx, "Dataset file found for org, patching it and creating new version",
				"orgId", orgId, "datasetFile", datasetFile)
		}
	}

	logger.DebugContext(ctx, "Successfully created full dataset")
	return nil
}

func (w *CreateFullDatasetWorker) handleFirstFullDataset(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Creating first full dataset", "orgId", orgId)

	clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, orgId.String())
	if err != nil {
		return errors.Wrap(err, "failed to get client db executor")
	}

	dataModel, err := w.repo.GetDataModel(ctx, exec, orgId.String(), false, false)
	if err != nil {
		return errors.Wrap(err, "failed to get data model")
	}

	now := time.Now()
	cursorEntityId := ""

	version := fmt.Sprintf("%s-001", now.Format("20060102"))
	fileName := fmt.Sprintf("%s-entities.ftm.json", version)
	fullDatasetFileName := fmt.Sprintf("%s/%s/%s", orgId.String(), FullDatasetFolderName, fileName)

	blob, err := w.blobRepository.OpenStream(ctx, w.bucketUrl, fullDatasetFileName, fileName)
	if err != nil {
		return errors.Wrap(err, "failed to open stream")
	}
	defer blob.Close()

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
			return errors.Wrap(err, "failed to list continuous screening last change by entity ids")
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
				return errors.Wrapf(models.NotFoundError,
					"table %s not found in data model", objectType)
			}
			if err := checkDataModelTableAndFieldsConfiguration(dataModelTable); err != nil {
				return errors.Wrap(err, "data model table is not correctly configured for the use case")
			}
			ingestedObjectsFromDb, err := w.ingestedDataReader.QueryIngestedObjectByInternalIds(
				ctx, clientDbExec, dataModelTable, objectInternalIds)
			if err != nil {
				return errors.Wrap(err, "failed to query ingested objects by internal ids")
			}
			if len(ingestedObjectsFromDb) != len(objectInternalIds) {
				return errors.Wrapf(models.NotFoundError,
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
				return errors.Wrapf(models.NotFoundError,
					"ingested object not found for object type %s and object internal id %s",
					deltaTrack.ObjectType, deltaTrack.ObjectInternalId)
			}
			datasetEntity := buildDatasetEntity(
				orgId,
				dataModel.Tables[deltaTrack.ObjectType],
				deltaTrack,
				ingestedObjectData,
			)
			entityJson, err := json.Marshal(datasetEntity)
			if err != nil {
				return errors.Wrap(err, "failed to marshal dataset entity")
			}
			_, err = blob.Write(append(entityJson, '\n'))
			if err != nil {
				return errors.Wrap(err, "failed to write dataset entity to blob")
			}
		}

		cursorEntityId = deltaTracks[len(deltaTracks)-1].EntityId
	}

	return nil
}

type datasetEntity struct {
	Id         string         `json:"id"`
	Schema     string         `json:"schema"`
	Datasets   []string       `json:"datasets"`
	Properties map[string]any `json:"properties"`
}

func buildDatasetEntity(orgId uuid.UUID, table models.Table,
	track models.ContinuousScreeningDeltaTrack, ingestedObjectData models.DataModelObject,
) datasetEntity {
	properties := make(map[string]any)
	for _, field := range table.Fields {
		if field.FTMProperty != nil {
			properties[field.FTMProperty.String()] = ingestedObjectData.Data[field.Name]
		}
	}

	return datasetEntity{
		Id:         track.EntityId,
		Schema:     table.FTMEntity.String(),
		Datasets:   []string{orgId.String()},
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
