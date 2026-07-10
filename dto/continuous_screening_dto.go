package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type ContinuousScreeningDatasetUpdateDto struct {
	Id          uuid.UUID `json:"id"`
	DatasetName string    `json:"dataset_name"`
	// Version is the stored (processed) version; LiveVersion is the current version reported by
	// the provider catalog. They differ when the stored data is not up to date.
	Version     string `json:"version"`
	LiveVersion string `json:"live_version"`
	Title       string `json:"title"`
	IsCurrent   bool   `json:"is_current"`
	TotalItems  int    `json:"total_items"`
	// Aggregated status across processing jobs for this dataset update ("processing",
	// "completed", "failed", ...).
	Status     string                                     `json:"status"`
	Completion ContinuousScreeningDatasetUpdateCompletion `json:"completion"`
	CreatedAt  time.Time                                  `json:"created_at"`
}

type ContinuousScreeningDatasetUpdateCompletion struct {
	Completed  int `json:"completed"`
	Processing int `json:"processing"`
	Pending    int `json:"pending"`
	Failed     int `json:"failed"`
	Total      int `json:"total"`
}

func AdaptContinuousScreeningDatasetUpdateDto(
	u models.ContinuousScreeningDatasetUpdateEnriched,
) ContinuousScreeningDatasetUpdateDto {
	return ContinuousScreeningDatasetUpdateDto{
		Id:          u.Id,
		DatasetName: u.DatasetName,
		Version:     u.Version,
		LiveVersion: u.LiveVersion,
		Title:       u.Title,
		IsCurrent:   u.IsCurrent,
		TotalItems:  u.TotalItems,
		Status:      u.Status.String(),
		Completion: ContinuousScreeningDatasetUpdateCompletion{
			Completed:  u.Completion.Completed,
			Processing: u.Completion.Processing,
			Pending:    u.Completion.Pending,
			Failed:     u.Completion.Failed,
			Total:      u.Completion.Total,
		},
		CreatedAt: u.CreatedAt,
	}
}

type ContinuousScreeningJobErrorDto struct {
	Details   json.RawMessage `json:"details"`
	CreatedAt time.Time       `json:"created_at"`
}

func AdaptContinuousScreeningJobErrorDto(e models.ContinuousScreeningJobError) ContinuousScreeningJobErrorDto {
	return ContinuousScreeningJobErrorDto{
		Details:   e.Details,
		CreatedAt: e.CreatedAt,
	}
}

type ContinuousScreeningUpdateJobDto struct {
	Id             uuid.UUID                        `json:"id"`
	Status         string                           `json:"status"`
	JobStart       time.Time                        `json:"job_start"`
	JobEnd         time.Time                        `json:"job_end"`
	ConfigName     string                           `json:"config_name"`
	Description    string                           `json:"description"`
	TotalItems     int                              `json:"total_items"`
	ReceptionTime  time.Time                        `json:"reception_time"`
	Version        string                           `json:"version"`
	ItemsProcessed *int                             `json:"items_processed"`
	Errors         []ContinuousScreeningJobErrorDto `json:"errors"`
}

func AdaptContinuousScreeningUpdateJobDto(
	j models.ContinuousScreeningUpdateJobSummary,
) ContinuousScreeningUpdateJobDto {
	return ContinuousScreeningUpdateJobDto{
		Id:             j.Id,
		Status:         j.Status.String(),
		JobStart:       j.JobStart,
		JobEnd:         j.JobEnd,
		ConfigName:     j.ConfigName,
		Description:    j.Description,
		TotalItems:     j.TotalItems,
		ReceptionTime:  j.ReceptionTime,
		Version:        j.Version,
		ItemsProcessed: j.ItemsProcessed,
		Errors:         pure_utils.Map(j.Errors, AdaptContinuousScreeningJobErrorDto),
	}
}

type ContinuousScreeningClientDataIndexingDto struct {
	Id             uuid.UUID                        `json:"id"`
	Status         string                           `json:"status"`
	JobStart       time.Time                        `json:"job_start"`
	TotalItems     int                              `json:"total_items"`
	ItemsProcessed *int                             `json:"items_processed"`
	Errors         []ContinuousScreeningJobErrorDto `json:"errors"`
}

func AdaptContinuousScreeningClientDataIndexingDto(
	j models.ContinuousScreeningClientDataIndexingSummary,
) ContinuousScreeningClientDataIndexingDto {
	return ContinuousScreeningClientDataIndexingDto{
		Id:             j.Id,
		Status:         j.Status.String(),
		JobStart:       j.JobStart,
		TotalItems:     j.TotalItems,
		ItemsProcessed: j.ItemsProcessed,
		Errors:         pure_utils.Map(j.Errors, AdaptContinuousScreeningJobErrorDto),
	}
}

type ContinuousScreeningDto struct {
	Id                                uuid.UUID                     `json:"id"`
	OrgId                             uuid.UUID                     `json:"org_id"`
	ContinuousScreeningConfigId       uuid.UUID                     `json:"continuous_screening_config_id"`
	ContinuousScreeningConfigStableId uuid.UUID                     `json:"continuous_screening_config_stable_id"`
	Provider                          string                        `json:"provider"`
	CaseId                            *uuid.UUID                    `json:"case_id"`
	ObjectType                        *string                       `json:"object_type,omitempty"`
	ObjectId                          *string                       `json:"object_id,omitempty"`
	ObjectInternalId                  *uuid.UUID                    `json:"object_internal_id,omitempty"`
	OpenSanctionEntityId              *string                       `json:"opensanction_entity_id,omitempty"`      //nolint:tagliatelle
	OpenSanctionEntityPayload         json.RawMessage               `json:"opensanction_entity_payload,omitempty"` //nolint:tagliatelle
	Status                            string                        `json:"status"`
	TriggerType                       string                        `json:"trigger_type"`
	Request                           ScreeningRequestDto           `json:"request"`
	Partial                           bool                          `json:"partial"`
	NumberOfMatches                   int                           `json:"number_of_matches"`
	Matches                           []ContinuousScreeningMatchDto `json:"matches"`
	CreatedAt                         time.Time                     `json:"created_at"`
	UpdatedAt                         time.Time                     `json:"updated_at"`
}

func AdaptContinuousScreeningDto(m models.ContinuousScreeningWithMatches) ContinuousScreeningDto {
	return ContinuousScreeningDto{
		Id:                                m.Id,
		OrgId:                             m.OrgId,
		ContinuousScreeningConfigId:       m.ContinuousScreeningConfigId,
		ContinuousScreeningConfigStableId: m.ContinuousScreeningConfigStableId,
		Provider:                          string(m.Provider),
		CaseId:                            m.CaseId,
		ObjectType:                        m.ObjectType,
		ObjectId:                          m.ObjectId,
		ObjectInternalId:                  m.ObjectInternalId,
		OpenSanctionEntityId:              m.OpenSanctionEntityId,
		OpenSanctionEntityPayload:         m.OpenSanctionEntityPayload,
		Status:                            m.Status.String(),
		TriggerType:                       m.TriggerType.String(),
		Request: ScreeningRequestDto{
			SearchInput: m.SearchInput,
		},
		Partial:         m.IsPartial,
		NumberOfMatches: m.NumberOfMatches,
		Matches:         pure_utils.Map(m.Matches, AdaptContinuousScreeningMatchDto),
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

type ContinuousScreeningMatchDto struct {
	Id                    uuid.UUID                  `json:"id"`
	ContinuousScreeningId uuid.UUID                  `json:"continuous_screening_id"`
	OpenSanctionEntityId  string                     `json:"opensanction_entity_id"` //nolint:tagliatelle
	ObjectType            string                     `json:"object_type"`
	ObjectId              string                     `json:"object_id"`
	Status                string                     `json:"status"`
	Payload               json.RawMessage            `json:"payload"`
	ReviewedBy            *uuid.UUID                 `json:"reviewed_by"`
	Comments              []ScreeningMatchCommentDto `json:"comments"`
	CreatedAt             time.Time                  `json:"created_at"`
	UpdatedAt             time.Time                  `json:"updated_at"`
}

func AdaptContinuousScreeningMatchDto(m models.ContinuousScreeningMatch) ContinuousScreeningMatchDto {
	var objectType string
	var objectId string
	if m.Metadata != nil {
		objectType = m.Metadata.ObjectType
		objectId = m.Metadata.ObjectId
	}

	return ContinuousScreeningMatchDto{
		Id:                    m.Id,
		ContinuousScreeningId: m.ContinuousScreeningId,
		OpenSanctionEntityId:  m.OpenSanctionEntityId,
		ObjectType:            objectType,
		ObjectId:              objectId,
		Status:                m.Status.String(),
		Payload:               m.Payload,
		ReviewedBy:            m.ReviewedBy,
		Comments:              pure_utils.Map(m.Comments, AdaptScreeningMatchCommentDto),
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
	}
}
