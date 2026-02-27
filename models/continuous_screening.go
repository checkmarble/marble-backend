package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ContinuousScreeningTriggerType int

const (
	ContinuousScreeningTriggerTypeUnknown ContinuousScreeningTriggerType = iota
	ContinuousScreeningTriggerTypeObjectAdded
	ContinuousScreeningTriggerTypeObjectUpdated
	ContinuousScreeningTriggerTypeDatasetUpdated
)

func ContinuousScreeningTriggerTypeFrom(s string) ContinuousScreeningTriggerType {
	switch s {
	case "object_added":
		return ContinuousScreeningTriggerTypeObjectAdded
	case "object_updated":
		return ContinuousScreeningTriggerTypeObjectUpdated
	case "dataset_updated":
		return ContinuousScreeningTriggerTypeDatasetUpdated
	}

	return ContinuousScreeningTriggerTypeUnknown
}

func (stt ContinuousScreeningTriggerType) String() string {
	switch stt {
	case ContinuousScreeningTriggerTypeObjectAdded:
		return "object_added"
	case ContinuousScreeningTriggerTypeObjectUpdated:
		return "object_updated"
	case ContinuousScreeningTriggerTypeDatasetUpdated:
		return "dataset_updated"
	}

	return "unknown"
}

type CreateContinuousScreeningObject struct {
	ObjectType     string
	ConfigStableId uuid.UUID
	ObjectId       string
	SkipScreen     bool
}

type DeleteContinuousScreeningObject struct {
	ObjectType     string
	ObjectId       string
	ConfigStableId uuid.UUID
}

type ContinuousScreening struct {
	Id                                uuid.UUID
	OrgId                             uuid.UUID
	ContinuousScreeningConfigId       uuid.UUID
	ContinuousScreeningConfigStableId uuid.UUID
	CaseId                            *uuid.UUID
	ObjectType                        *string
	ObjectId                          *string
	ObjectInternalId                  *uuid.UUID
	OpenSanctionEntityId              *string
	OpenSanctionEntityPayload         json.RawMessage
	OpenSanctionEntityEnriched        bool
	Status                            ScreeningStatus
	TriggerType                       ContinuousScreeningTriggerType
	SearchInput                       json.RawMessage
	IsPartial                         bool
	NumberOfMatches                   int

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (cs *ContinuousScreening) IsObjectTriggered() bool {
	return cs.TriggerType == ContinuousScreeningTriggerTypeObjectAdded ||
		cs.TriggerType == ContinuousScreeningTriggerTypeObjectUpdated
}

func (cs *ContinuousScreening) IsDatasetTriggered() bool {
	return cs.TriggerType == ContinuousScreeningTriggerTypeDatasetUpdated
}

type UpdateContinuousScreeningInput struct {
	Status          *ScreeningStatus
	IsPartial       *bool
	NumberOfMatches *int
	CaseId          *uuid.UUID
}

type CreateContinuousScreening struct {
	Screening                 ScreeningWithMatches
	Config                    ContinuousScreeningConfig
	ObjectType                *string
	ObjectId                  *string
	ObjectInternalId          *uuid.UUID
	OpenSanctionEntityId      *string
	OpenSanctionEntityPayload json.RawMessage
	TriggerType               ContinuousScreeningTriggerType
}

type ContinuousScreeningMatch struct {
	Id                    uuid.UUID
	ContinuousScreeningId uuid.UUID
	OpenSanctionEntityId  string
	Status                ScreeningMatchStatus
	Payload               json.RawMessage
	Enriched              bool
	ReviewedBy            *uuid.UUID
	Metadata              *EntityNoteMetadata

	CreatedAt time.Time
	UpdatedAt time.Time
}

type ContinuousScreeningWithMatches struct {
	ContinuousScreening

	Matches []ContinuousScreeningMatch
}

const ContinuousScreeningSortingCreatedAt SortingField = SortingFieldCreatedAt

// Used as a prefix for the screening indexer endpoints and for the URL building functions
const ScreeningIndexerKey = "screening-indexer"

type ContinuousScreeningMonitoredObject struct {
	Id             uuid.UUID
	ObjectType     string
	ObjectId       string
	ConfigStableId uuid.UUID
	CreatedAt      time.Time
}

type ContinuousScreeningDataModelMapping struct {
	Entity     string
	Properties map[string]string
}

type ListMonitoredObjectsFilters struct {
	ObjectTypes     []string    // Optional: filter by object types
	ObjectIds       []string    // Optional: filter by object IDs
	ConfigStableIds []uuid.UUID // Optional: filter by config stable IDs
	StartDate       *time.Time  // Optional: filter objects created on/after this date
	EndDate         *time.Time  // Optional: filter objects created on/before this date
}

type ContinuousScreeningDatasetUpdate struct {
	Id            uuid.UUID
	DatasetName   string
	Version       string
	DeltaFilePath string
	TotalItems    int
	CreatedAt     time.Time
}

type CreateContinuousScreeningDatasetUpdate struct {
	DatasetName   string
	Version       string
	DeltaFilePath string // In our storage
	TotalItems    int
}

type ContinuousScreeningUpdateJobStatus int

const (
	ContinuousScreeningUpdateJobStatusUnknown ContinuousScreeningUpdateJobStatus = iota
	ContinuousScreeningUpdateJobStatusPending
	ContinuousScreeningUpdateJobStatusProcessing
	ContinuousScreeningUpdateJobStatusCompleted
	ContinuousScreeningUpdateJobStatusFailed
)

func ContinuousScreeningUpdateJobStatusFrom(s string) ContinuousScreeningUpdateJobStatus {
	switch s {
	case "pending":
		return ContinuousScreeningUpdateJobStatusPending
	case "processing":
		return ContinuousScreeningUpdateJobStatusProcessing
	case "completed":
		return ContinuousScreeningUpdateJobStatusCompleted
	case "failed":
		return ContinuousScreeningUpdateJobStatusFailed
	}
	return ContinuousScreeningUpdateJobStatusUnknown
}

func (s ContinuousScreeningUpdateJobStatus) String() string {
	switch s {
	case ContinuousScreeningUpdateJobStatusPending:
		return "pending"
	case ContinuousScreeningUpdateJobStatusProcessing:
		return "processing"
	case ContinuousScreeningUpdateJobStatusCompleted:
		return "completed"
	case ContinuousScreeningUpdateJobStatusFailed:
		return "failed"
	}
	return "unknown"
}

// ContinuousScreeningUpdateJob represents a job to process dataset updates
type ContinuousScreeningUpdateJob struct {
	Id              uuid.UUID
	DatasetUpdateId uuid.UUID
	ConfigId        uuid.UUID
	OrgId           uuid.UUID
	Status          ContinuousScreeningUpdateJobStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateContinuousScreeningUpdateJob struct {
	DatasetUpdateId uuid.UUID
	ConfigId        uuid.UUID
	OrgId           uuid.UUID
}

type EnrichedContinuousScreeningUpdateJob struct {
	ContinuousScreeningUpdateJob
	Config        ContinuousScreeningConfig
	DatasetUpdate ContinuousScreeningDatasetUpdate
}

type ContinuousScreeningUpdateJobWithProgress struct {
	ContinuousScreeningUpdateJob
	DatasetName    string
	DatasetVersion string
	TotalItems     int
	ItemsProcessed int
}

type ContinuousScreeningDeltaTrackWithFile struct {
	ContinuousScreeningDeltaTrack
	DatasetFile *ContinuousScreeningDatasetFile
}

// ContinuousScreeningJobOffset tracks the progress of processing a dataset update
type ContinuousScreeningJobOffset struct {
	Id             uuid.UUID
	UpdateJobId    uuid.UUID
	ByteOffset     int64
	ItemsProcessed int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateContinuousScreeningJobOffset struct {
	UpdateJobId    uuid.UUID
	ByteOffset     int64
	ItemsProcessed int
}

// ContinuousScreeningJobError tracks errors encountered during job processing
type ContinuousScreeningJobError struct {
	Id          uuid.UUID
	UpdateJobId uuid.UUID
	Details     json.RawMessage
	CreatedAt   time.Time
}

type CreateContinuousScreeningJobError struct {
	UpdateJobId uuid.UUID
	Details     json.RawMessage
}

type ContinuousScreeningEnqueueObjectUpdateTask struct {
	MonitoringId       uuid.UUID
	PreviousInternalId string
	NewInternalId      string
}

type DeltaTrackOperation int

const (
	DeltaTrackOperationUnknown DeltaTrackOperation = iota
	DeltaTrackOperationAdd
	DeltaTrackOperationUpdate
	DeltaTrackOperationDelete
)

func (o DeltaTrackOperation) String() string {
	switch o {
	case DeltaTrackOperationAdd:
		return "add"
	case DeltaTrackOperationUpdate:
		return "update"
	case DeltaTrackOperationDelete:
		return "delete"
	default:
		return "unknown"
	}
}

func DeltaTrackOperationFrom(s string) DeltaTrackOperation {
	switch s {
	case "add":
		return DeltaTrackOperationAdd
	case "update":
		return DeltaTrackOperationUpdate
	case "delete":
		return DeltaTrackOperationDelete
	default:
		return DeltaTrackOperationUnknown
	}
}

type ContinuousScreeningDatasetFileType int

const (
	ContinuousScreeningDatasetFileTypeUnknown ContinuousScreeningDatasetFileType = iota
	ContinuousScreeningDatasetFileTypeFull
	ContinuousScreeningDatasetFileTypeDelta
)

func ContinuousScreeningDatasetFileTypeFrom(s string) ContinuousScreeningDatasetFileType {
	switch s {
	case "full":
		return ContinuousScreeningDatasetFileTypeFull
	case "delta":
		return ContinuousScreeningDatasetFileTypeDelta
	default:
		return ContinuousScreeningDatasetFileTypeUnknown
	}
}

func (ft ContinuousScreeningDatasetFileType) String() string {
	switch ft {
	case ContinuousScreeningDatasetFileTypeFull:
		return "full"
	case ContinuousScreeningDatasetFileTypeDelta:
		return "delta"
	default:
		return "unknown"
	}
}

type ContinuousScreeningDatasetFile struct {
	Id        uuid.UUID
	OrgId     uuid.UUID
	FileType  ContinuousScreeningDatasetFileType
	Version   string
	FilePath  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ContinuousScreeningDeltaTrack struct {
	Id               uuid.UUID
	OrgId            uuid.UUID
	ObjectType       string
	ObjectId         string
	ObjectInternalId *uuid.UUID
	EntityId         string // ID in Screening Provider DB
	Operation        DeltaTrackOperation
	DatasetFileId    *uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type EntityNoteMetadata struct {
	ObjectId   string `json:"object_id"`
	ObjectType string `json:"object_type"`
}

type CreateContinuousScreeningDeltaTrack struct {
	OrgId            uuid.UUID
	ObjectType       string
	ObjectId         string
	ObjectInternalId *uuid.UUID
	EntityId         string
	Operation        DeltaTrackOperation
}

type CreateContinuousScreeningDatasetFile struct {
	OrgId    uuid.UUID
	FileType ContinuousScreeningDatasetFileType
	Version  string
	FilePath string
}
