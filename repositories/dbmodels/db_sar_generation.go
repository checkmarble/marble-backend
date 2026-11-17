package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const (
	TABLE_SAR_GENERATION_JOBS      = "sar_generation_jobs"
	TABLE_SAR_GENERATION_TASKS     = "sar_generation_tasks"
	TABLE_SAR_DECLARATION_QUESTION = "sar_declaration_questions"
)

type DBSarGenerationJob struct {
	Id            uuid.UUID       `db:"id"`
	DeclarationId uuid.UUID       `db:"declaration_id"`
	Kind          string          `db:"kind"`
	Status        string          `db:"status"`
	Input         json.RawMessage `db:"input"`
	Error         *string         `db:"error"`
	CreatedBy     *uuid.UUID      `db:"created_by"`
	Attempt       int             `db:"attempt"`
	// RateLimitedSince is the start of the current provider rate-limit streak.
	RateLimitedSince *time.Time `db:"rate_limited_since"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

var SarGenerationJobColumns = utils.ColumnList[DBSarGenerationJob]()

func AdaptSarGenerationJob(db DBSarGenerationJob) (models.SarGenerationJob, error) {
	return models.SarGenerationJob{
		Id:               db.Id,
		DeclarationId:    db.DeclarationId,
		Kind:             models.SarJobKind(db.Kind),
		Status:           models.SarJobStatus(db.Status),
		Input:            db.Input,
		Error:            derefString(db.Error),
		CreatedBy:        db.CreatedBy,
		Attempt:          db.Attempt,
		RateLimitedSince: db.RateLimitedSince,
		CreatedAt:        db.CreatedAt,
		UpdatedAt:        db.UpdatedAt,
	}, nil
}

// DBSarGenerationTask does not include the trace column (loaded on demand).
type DBSarGenerationTask struct {
	Id         uuid.UUID       `db:"id"`
	JobId      uuid.UUID       `db:"job_id"`
	Kind       string          `db:"kind"`
	TargetPath *string         `db:"target_path"`
	Input      json.RawMessage `db:"input"`
	InputHash  string          `db:"input_hash"`
	DependsOn  []uuid.UUID     `db:"depends_on"`
	Status     string          `db:"status"`
	Attempts   int             `db:"attempts"`
	Output     json.RawMessage `db:"output"`
	Error      *string         `db:"error"`
	StartedAt  *time.Time      `db:"started_at"`
	FinishedAt *time.Time      `db:"finished_at"`
}

var SarGenerationTaskColumns = utils.ColumnList[DBSarGenerationTask]()

func AdaptSarGenerationTask(db DBSarGenerationTask) (models.SarGenerationTask, error) {
	dependsOn := db.DependsOn
	if dependsOn == nil {
		dependsOn = []uuid.UUID{}
	}
	return models.SarGenerationTask{
		Id:         db.Id,
		JobId:      db.JobId,
		Kind:       db.Kind,
		TargetPath: derefString(db.TargetPath),
		Input:      db.Input,
		InputHash:  db.InputHash,
		DependsOn:  dependsOn,
		Status:     models.SarJobStatus(db.Status),
		Attempts:   db.Attempts,
		Output:     db.Output,
		Error:      derefString(db.Error),
		StartedAt:  db.StartedAt,
		FinishedAt: db.FinishedAt,
	}, nil
}

// DBSarGenerationTaskSummary is the light projection of a task (no input,
// output or trace).
type DBSarGenerationTaskSummary struct {
	Id         uuid.UUID  `db:"id"`
	JobId      uuid.UUID  `db:"job_id"`
	Kind       string     `db:"kind"`
	TargetPath *string    `db:"target_path"`
	Status     string     `db:"status"`
	Attempts   int        `db:"attempts"`
	Error      *string    `db:"error"`
	StartedAt  *time.Time `db:"started_at"`
	FinishedAt *time.Time `db:"finished_at"`
}

var SarGenerationTaskSummaryColumns = utils.ColumnList[DBSarGenerationTaskSummary]()

func AdaptSarGenerationTaskSummary(db DBSarGenerationTaskSummary) (models.SarGenerationTask, error) {
	return models.SarGenerationTask{
		Id:         db.Id,
		JobId:      db.JobId,
		Kind:       db.Kind,
		TargetPath: derefString(db.TargetPath),
		Status:     models.SarJobStatus(db.Status),
		Attempts:   db.Attempts,
		Error:      derefString(db.Error),
		StartedAt:  db.StartedAt,
		FinishedAt: db.FinishedAt,
	}, nil
}

type DBSarQuestion struct {
	Id            uuid.UUID       `db:"id"`
	DeclarationId uuid.UUID       `db:"declaration_id"`
	TaskId        *uuid.UUID      `db:"task_id"`
	Kind          string          `db:"kind"`
	Severity      *string         `db:"severity"`
	Paths         []string        `db:"paths"`
	Text          string          `db:"text"`
	Options       json.RawMessage `db:"options"`
	Status        string          `db:"status"`
	CreatedAt     time.Time       `db:"created_at"`
	ClosedAt      *time.Time      `db:"closed_at"`
}

// "text" is a SQL type name but a valid column name; no escaping needed.
var SarQuestionColumns = utils.ColumnList[DBSarQuestion]()

func AdaptSarQuestion(db DBSarQuestion) (models.SarQuestion, error) {
	return models.SarQuestion{
		Id:            db.Id,
		DeclarationId: db.DeclarationId,
		TaskId:        db.TaskId,
		Kind:          db.Kind,
		Severity:      derefString(db.Severity),
		Paths:         db.Paths,
		Text:          db.Text,
		Options:       db.Options,
		Status:        db.Status,
		CreatedAt:     db.CreatedAt,
		ClosedAt:      db.ClosedAt,
	}, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
