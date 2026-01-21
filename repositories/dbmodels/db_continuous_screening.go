package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_CONTINUOUS_SCREENINGS = "continuous_screenings"

var SelectContinuousScreeningColumn = utils.ColumnList[DBContinuousScreening]()

type DBContinuousScreening struct {
	Id                                uuid.UUID       `db:"id"`
	OrgId                             uuid.UUID       `db:"org_id"`
	ContinuousScreeningConfigId       uuid.UUID       `db:"continuous_screening_config_id"`
	ContinuousScreeningConfigStableId uuid.UUID       `db:"continuous_screening_config_stable_id"`
	CaseId                            *uuid.UUID      `db:"case_id"`
	ObjectType                        *string         `db:"object_type"`
	ObjectId                          *string         `db:"object_id"`
	ObjectInternalId                  *uuid.UUID      `db:"object_internal_id"`
	OpenSanctionEntityId              *string         `db:"opensanction_entity_id"`
	OpenSanctionEntityPayload         json.RawMessage `db:"opensanction_entity_payload"`
	OpenSanctionEntityEnriched        bool            `db:"opensanction_entity_enriched"`
	Status                            string          `db:"status"`
	TriggerType                       string          `db:"trigger_type"`
	SearchInput                       json.RawMessage `db:"search_input"`
	IsPartial                         bool            `db:"is_partial"`
	NumberOfMatches                   int             `db:"number_of_matches"`
	CreatedAt                         time.Time       `db:"created_at"`
	UpdatedAt                         time.Time       `db:"updated_at"`
}

func AdaptContinuousScreening(db DBContinuousScreening) (models.ContinuousScreening, error) {
	return models.ContinuousScreening{
		Id:                                db.Id,
		OrgId:                             db.OrgId,
		ContinuousScreeningConfigId:       db.ContinuousScreeningConfigId,
		ContinuousScreeningConfigStableId: db.ContinuousScreeningConfigStableId,
		CaseId:                            db.CaseId,
		ObjectType:                        db.ObjectType,
		ObjectId:                          db.ObjectId,
		ObjectInternalId:                  db.ObjectInternalId,
		OpenSanctionEntityId:              db.OpenSanctionEntityId,
		OpenSanctionEntityPayload:         db.OpenSanctionEntityPayload,
		OpenSanctionEntityEnriched:        db.OpenSanctionEntityEnriched,
		Status:                            models.ScreeningStatusFrom(db.Status),
		TriggerType:                       models.ContinuousScreeningTriggerTypeFrom(db.TriggerType),
		SearchInput:                       db.SearchInput,
		IsPartial:                         db.IsPartial,
		NumberOfMatches:                   db.NumberOfMatches,
		CreatedAt:                         db.CreatedAt,
		UpdatedAt:                         db.UpdatedAt,
	}, nil
}

const TABLE_CONTINUOUS_SCREENING_MATCHES = "continuous_screening_matches"

var SelectContinuousScreeningMatchesColumn = utils.ColumnList[DBContinuousScreeningMatches]()

type DBContinuousScreeningMatches struct {
	Id                    uuid.UUID       `db:"id"`
	ContinuousScreeningId uuid.UUID       `db:"continuous_screening_id"`
	OpenSanctionEntityId  string          `db:"opensanction_entity_id"`
	Status                string          `db:"status"`
	Payload               json.RawMessage `db:"payload"`
	Enriched              bool            `db:"enriched"`
	ReviewedBy            *uuid.UUID      `db:"reviewed_by"`
	CreatedAt             time.Time       `db:"created_at"`
	UpdatedAt             time.Time       `db:"updated_at"`
}

func AdaptContinuousScreeningMatch(dto DBContinuousScreeningMatches) (models.ContinuousScreeningMatch, error) {
	var metadata *models.EntityNoteMetadata
	// Optimization: Use a specific struct to ignore all other properties and avoid map allocations
	var payload struct {
		Properties struct {
			Notes []string `json:"notes"`
		} `json:"properties"`
	}

	if err := json.Unmarshal(dto.Payload, &payload); err == nil {
		for _, note := range payload.Properties.Notes {
			var meta models.EntityNoteMetadata
			if err := json.Unmarshal([]byte(note), &meta); err == nil {
				metadata = &meta
				break
			}
		}
	}

	return models.ContinuousScreeningMatch{
		Id:                    dto.Id,
		ContinuousScreeningId: dto.ContinuousScreeningId,
		OpenSanctionEntityId:  dto.OpenSanctionEntityId,
		Status:                models.ScreeningMatchStatusFrom(dto.Status),
		Payload:               dto.Payload,
		Enriched:              dto.Enriched,
		ReviewedBy:            dto.ReviewedBy,
		Metadata:              metadata,
		CreatedAt:             dto.CreatedAt,
		UpdatedAt:             dto.UpdatedAt,
	}, nil
}

type DBContinuousScreeningWithMatches struct {
	DBContinuousScreening
	Matches []DBContinuousScreeningMatches `db:"matches"`
}

func AdaptContinuousScreeningWithMatches(dto DBContinuousScreeningWithMatches) (models.ContinuousScreeningWithMatches, error) {
	matches := make([]models.ContinuousScreeningMatch, 0, len(dto.Matches))
	for _, match := range dto.Matches {
		m, err := AdaptContinuousScreeningMatch(match)
		if err != nil {
			return models.ContinuousScreeningWithMatches{}, err
		}

		matches = append(matches, m)
	}

	sm, err := AdaptContinuousScreening(dto.DBContinuousScreening)
	if err != nil {
		return models.ContinuousScreeningWithMatches{}, err
	}

	return models.ContinuousScreeningWithMatches{
		ContinuousScreening: sm,
		Matches:             matches,
	}, nil
}

const TABLE_CONTINUOUS_SCREENING_MONITORED_OBJECTS = "_monitored_objects"

type DBContinuousScreeningMonitoredObject struct {
	Id             uuid.UUID `db:"id"`
	ObjectType     string    `db:"object_type"`
	ObjectId       string    `db:"object_id"`
	ConfigStableId uuid.UUID `db:"config_stable_id"`
	CreatedAt      time.Time `db:"created_at"`
}

var SelectContinuousScreeningMonitoredObjectColumn = utils.ColumnList[DBContinuousScreeningMonitoredObject]()

func AdaptContinuousScreeningMonitoredObject(dto DBContinuousScreeningMonitoredObject) (models.ContinuousScreeningMonitoredObject, error) {
	return models.ContinuousScreeningMonitoredObject{
		Id:             dto.Id,
		ObjectType:     dto.ObjectType,
		ObjectId:       dto.ObjectId,
		ConfigStableId: dto.ConfigStableId,
		CreatedAt:      dto.CreatedAt,
	}, nil
}

const TABLE_CONTINUOUS_SCREENING_DATASET_UPDATES = "continuous_screening_dataset_updates"

var SelectContinuousScreeningDatasetUpdateColumn = utils.ColumnList[DBContinuousScreeningDatasetUpdate]()

type DBContinuousScreeningDatasetUpdate struct {
	Id            uuid.UUID `db:"id"`
	DatasetName   string    `db:"dataset_name"`
	Version       string    `db:"version"`
	DeltaFilePath string    `db:"delta_file_path"`
	TotalItems    int       `db:"total_items"`
	CreatedAt     time.Time `db:"created_at"`
}

func AdaptContinuousScreeningDatasetUpdate(dto DBContinuousScreeningDatasetUpdate) (models.ContinuousScreeningDatasetUpdate, error) {
	return models.ContinuousScreeningDatasetUpdate{
		Id:            dto.Id,
		DatasetName:   dto.DatasetName,
		Version:       dto.Version,
		DeltaFilePath: dto.DeltaFilePath,
		TotalItems:    dto.TotalItems,
		CreatedAt:     dto.CreatedAt,
	}, nil
}

const TABLE_CONTINUOUS_SCREENING_UPDATE_JOBS = "continuous_screening_update_jobs"

var SelectContinuousScreeningUpdateJobColumn = utils.ColumnList[DBContinuousScreeningUpdateJob]()

type DBContinuousScreeningUpdateJob struct {
	Id                                 uuid.UUID `db:"id"`
	ContinuousScreeningDatasetUpdateId uuid.UUID `db:"continuous_screening_dataset_update_id"`
	ContinuousScreeningConfigId        uuid.UUID `db:"continuous_screening_config_id"`
	OrgId                              uuid.UUID `db:"org_id"`
	Status                             string    `db:"status"`
	CreatedAt                          time.Time `db:"created_at"`
	UpdatedAt                          time.Time `db:"updated_at"`
}

func AdaptContinuousScreeningUpdateJob(dto DBContinuousScreeningUpdateJob) (models.ContinuousScreeningUpdateJob, error) {
	return models.ContinuousScreeningUpdateJob{
		Id:              dto.Id,
		DatasetUpdateId: dto.ContinuousScreeningDatasetUpdateId,
		ConfigId:        dto.ContinuousScreeningConfigId,
		OrgId:           dto.OrgId,
		Status:          models.ContinuousScreeningUpdateJobStatusFrom(dto.Status),
		CreatedAt:       dto.CreatedAt,
		UpdatedAt:       dto.UpdatedAt,
	}, nil
}

type DBEnrichedContinuousScreeningUpdateJob struct {
	UpdateJob     DBContinuousScreeningUpdateJob     `db:"update_job"`
	Config        DBContinuousScreeningConfig        `db:"config"`
	DatasetUpdate DBContinuousScreeningDatasetUpdate `db:"dataset_update"`
}

func AdaptEnrichedContinuousScreeningUpdateJob(dto DBEnrichedContinuousScreeningUpdateJob) (models.EnrichedContinuousScreeningUpdateJob, error) {
	config, err := AdaptContinuousScreeningConfig(dto.Config)
	if err != nil {
		return models.EnrichedContinuousScreeningUpdateJob{}, err
	}
	datasetUpdate, err := AdaptContinuousScreeningDatasetUpdate(dto.DatasetUpdate)
	if err != nil {
		return models.EnrichedContinuousScreeningUpdateJob{}, err
	}
	updateJob, err := AdaptContinuousScreeningUpdateJob(dto.UpdateJob)
	if err != nil {
		return models.EnrichedContinuousScreeningUpdateJob{}, err
	}
	return models.EnrichedContinuousScreeningUpdateJob{
		ContinuousScreeningUpdateJob: updateJob,
		Config:                       config,
		DatasetUpdate:                datasetUpdate,
	}, nil
}

const TABLE_CONTINUOUS_SCREENING_JOB_OFFSETS = "continuous_screening_job_offsets"

var SelectContinuousScreeningJobOffsetColumn = utils.ColumnList[DBContinuousScreeningJobOffset]()

type DBContinuousScreeningJobOffset struct {
	Id                             uuid.UUID `db:"id"`
	ContinuousScreeningUpdateJobId uuid.UUID `db:"continuous_screening_update_job_id"`
	ByteOffset                     int64     `db:"byte_offset"`
	ItemsProcessed                 int       `db:"items_processed"` // Number of items processed
	CreatedAt                      time.Time `db:"created_at"`
	UpdatedAt                      time.Time `db:"updated_at"`
}

func AdaptContinuousScreeningJobOffset(dto DBContinuousScreeningJobOffset) (models.ContinuousScreeningJobOffset, error) {
	return models.ContinuousScreeningJobOffset{
		Id:             dto.Id,
		UpdateJobId:    dto.ContinuousScreeningUpdateJobId,
		ByteOffset:     dto.ByteOffset,
		ItemsProcessed: dto.ItemsProcessed,
		CreatedAt:      dto.CreatedAt,
		UpdatedAt:      dto.UpdatedAt,
	}, nil
}

const TABLE_CONTINUOUS_SCREENING_JOB_ERRORS = "continuous_screening_job_errors"

var SelectContinuousScreeningJobErrorColumn = utils.ColumnList[DBContinuousScreeningJobError]()

type DBContinuousScreeningJobError struct {
	Id                             uuid.UUID       `db:"id"`
	ContinuousScreeningUpdateJobId uuid.UUID       `db:"continuous_screening_update_job_id"`
	Details                        json.RawMessage `db:"details"`
	CreatedAt                      time.Time       `db:"created_at"`
}

func AdaptContinuousScreeningJobError(dto DBContinuousScreeningJobError) (models.ContinuousScreeningJobError, error) {
	return models.ContinuousScreeningJobError{
		Id:          dto.Id,
		UpdateJobId: dto.ContinuousScreeningUpdateJobId,
		Details:     dto.Details,
		CreatedAt:   dto.CreatedAt,
	}, nil
}

type DBContinuousScreeningDatasetFile struct {
	Id        uuid.UUID `db:"id"`
	OrgId     uuid.UUID `db:"org_id"`
	FileType  string    `db:"file_type"`
	Version   string    `db:"version"`
	FilePath  string    `db:"file_path"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

const TABLE_CONTINUOUS_SCREENING_DATASET_FILES = "continuous_screening_dataset_files"

var SelectContinuousScreeningDatasetFileColumn = utils.ColumnList[DBContinuousScreeningDatasetFile]()

func AdaptContinuousScreeningDatasetFile(dto DBContinuousScreeningDatasetFile) (models.ContinuousScreeningDatasetFile, error) {
	return models.ContinuousScreeningDatasetFile{
		Id:        dto.Id,
		OrgId:     dto.OrgId,
		FileType:  models.ContinuousScreeningDatasetFileTypeFrom(dto.FileType),
		Version:   dto.Version,
		FilePath:  dto.FilePath,
		CreatedAt: dto.CreatedAt,
		UpdatedAt: dto.UpdatedAt,
	}, nil
}

type DBContinuousScreeningDeltaTrack struct {
	Id               uuid.UUID  `db:"id"`
	OrgId            uuid.UUID  `db:"org_id"`
	ObjectType       string     `db:"object_type"`
	ObjectId         string     `db:"object_id"`
	ObjectInternalId *uuid.UUID `db:"object_internal_id"`
	EntityId         string     `db:"entity_id"`
	Operation        string     `db:"operation"`
	DatasetFileId    *uuid.UUID `db:"dataset_file_id"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

const TABLE_CONTINUOUS_SCREENING_DELTA_TRACKS = "continuous_screening_delta_tracks"

var SelectContinuousScreeningDeltaTrackColumn = utils.ColumnList[DBContinuousScreeningDeltaTrack]()

func AdaptContinuousScreeningDeltaTrack(dto DBContinuousScreeningDeltaTrack) (models.ContinuousScreeningDeltaTrack, error) {
	return models.ContinuousScreeningDeltaTrack{
		Id:               dto.Id,
		OrgId:            dto.OrgId,
		ObjectType:       dto.ObjectType,
		ObjectId:         dto.ObjectId,
		ObjectInternalId: dto.ObjectInternalId,
		EntityId:         dto.EntityId,
		Operation:        models.DeltaTrackOperationFrom(dto.Operation),
		DatasetFileId:    dto.DatasetFileId,
		CreatedAt:        dto.CreatedAt,
		UpdatedAt:        dto.UpdatedAt,
	}, nil
}
