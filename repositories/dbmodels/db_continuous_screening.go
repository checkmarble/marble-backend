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
	ObjectType                        string          `db:"object_type"`
	ObjectId                          string          `db:"object_id"`
	ObjectInternalId                  uuid.UUID       `db:"object_internal_id"`
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
	ReviewedBy            *uuid.UUID      `db:"reviewed_by"`
	CreatedAt             time.Time       `db:"created_at"`
	UpdatedAt             time.Time       `db:"updated_at"`
}

func AdaptContinuousScreeningMatch(dto DBContinuousScreeningMatches) (models.ContinuousScreeningMatch, error) {
	return models.ContinuousScreeningMatch{
		Id:                    dto.Id,
		ContinuousScreeningId: dto.ContinuousScreeningId,
		OpenSanctionEntityId:  dto.OpenSanctionEntityId,
		Status:                models.ScreeningMatchStatusFrom(dto.Status),
		Payload:               dto.Payload,
		ReviewedBy:            dto.ReviewedBy,
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

type DBContinuousScreeningMonitoredObject struct {
	Id             uuid.UUID `db:"id"`
	ObjectId       string    `db:"object_id"`
	ConfigStableId uuid.UUID `db:"config_stable_id"`
	CreatedAt      time.Time `db:"created_at"`
}

var SelectContinuousScreeningMonitoredObjectColumn = utils.ColumnList[DBContinuousScreeningMonitoredObject]()

func AdaptContinuousScreeningMonitoredObject(dto DBContinuousScreeningMonitoredObject) (models.ContinuousScreeningMonitoredObject, error) {
	return models.ContinuousScreeningMonitoredObject{
		Id:             dto.Id,
		ObjectId:       dto.ObjectId,
		ConfigStableId: dto.ConfigStableId,
		CreatedAt:      dto.CreatedAt,
	}, nil
}
