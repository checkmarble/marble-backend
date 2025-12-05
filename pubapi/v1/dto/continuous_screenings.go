package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type ContinuousScreeningRequest struct {
	SearchInput json.RawMessage `json:"search_input"`
}

type ContinuousScreeningMatch struct {
	Id                    uuid.UUID       `json:"id"`
	ContinuousScreeningId uuid.UUID       `json:"continuous_screening_id"`
	OpenSanctionEntityId  string          `json:"opensanction_entity_id"` //nolint:tagliatelle
	Status                string          `json:"status"`
	Payload               json.RawMessage `json:"payload"`
	ReviewedBy            *uuid.UUID      `json:"reviewed_by"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

func AdaptContinuousScreeningMatch(m models.ContinuousScreeningMatch) ContinuousScreeningMatch {
	return ContinuousScreeningMatch{
		Id:                    m.Id,
		ContinuousScreeningId: m.ContinuousScreeningId,
		OpenSanctionEntityId:  m.OpenSanctionEntityId,
		Status:                m.Status.String(),
		Payload:               m.Payload,
		ReviewedBy:            m.ReviewedBy,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
	}
}

type ContinuousScreening struct {
	Id                                uuid.UUID                  `json:"id"`
	OrgId                             uuid.UUID                  `json:"org_id"`
	ContinuousScreeningConfigId       uuid.UUID                  `json:"continuous_screening_config_id"`
	ContinuousScreeningConfigStableId uuid.UUID                  `json:"continuous_screening_config_stable_id"`
	CaseId                            *uuid.UUID                 `json:"case_id"`
	ObjectType                        string                     `json:"object_type"`
	ObjectId                          string                     `json:"object_id"`
	ObjectInternalId                  uuid.UUID                  `json:"object_internal_id"`
	Status                            string                     `json:"status"`
	TriggerType                       string                     `json:"trigger_type"`
	Request                           ContinuousScreeningRequest `json:"request"`
	Partial                           bool                       `json:"partial"`
	NumberOfMatches                   int                        `json:"number_of_matches"`
	Matches                           []ContinuousScreeningMatch `json:"matches"`
	CreatedAt                         time.Time                  `json:"created_at"`
	UpdatedAt                         time.Time                  `json:"updated_at"`
}

func AdaptContinuousScreening(m models.ContinuousScreeningWithMatches) ContinuousScreening {
	return ContinuousScreening{
		Id:                                m.Id,
		OrgId:                             m.OrgId,
		ContinuousScreeningConfigId:       m.ContinuousScreeningConfigId,
		ContinuousScreeningConfigStableId: m.ContinuousScreeningConfigStableId,
		CaseId:                            m.CaseId,
		ObjectType:                        m.ObjectType,
		ObjectId:                          m.ObjectId,
		ObjectInternalId:                  m.ObjectInternalId,
		Status:                            m.Status.String(),
		TriggerType:                       m.TriggerType.String(),
		Request: ContinuousScreeningRequest{
			SearchInput: m.SearchInput,
		},
		Partial:         m.IsPartial,
		NumberOfMatches: m.NumberOfMatches,
		Matches:         pure_utils.Map(m.Matches, AdaptContinuousScreeningMatch),
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}
