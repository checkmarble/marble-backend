package repositories

import (
	"context"
	"encoding/json"
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"time"
)

type TracfinAIRepository interface {
	InvalidateTracfinAIActive(context.Context, Executor, string, string) error
	CreateTracfinAIRun(context.Context, Executor, models.TracfinAIRun) error
	GetTracfinAIRun(context.Context, Executor, string, string, string) (models.TracfinAIRun, error)
	SaveTracfinAIRun(context.Context, Executor, models.TracfinAIRun, int64) error
	ListTracfinAIRemarks(context.Context, Executor, string, string) ([]models.TracfinAIRemark, error)
	ListTracfinAIRuns(context.Context, Executor, string, string, int) ([]models.TracfinAIRun, error)
}

func (repo *MarbleDbRepository) CreateTracfinAIRun(ctx context.Context, exec Executor, r models.TracfinAIRun) error {
	if err := validateTracfinDraftScope(exec, r.OrgID, r.CaseID); err != nil {
		return err
	}
	if r.ID == "" || r.ActorID == "" || r.Revision != 1 {
		return models.BadParameterError
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return execTracfinWrite(ctx, exec, NewQueryBuilder().Insert("tracfin_ai_runs").Columns("id", "org_id", "case_id", "actor_id", "status", "content").Values(r.ID, r.OrgID, r.CaseID, r.ActorID, r.Status, raw).Suffix("ON CONFLICT DO NOTHING"))
}

type dbTracfinAIRun struct {
	Content  json.RawMessage `db:"content"`
	Revision int64           `db:"revision"`
}

func (repo *MarbleDbRepository) GetTracfinAIRun(ctx context.Context, exec Executor, orgID, caseID, id string) (models.TracfinAIRun, error) {
	if err := validateTracfinDraftScope(exec, orgID, caseID); err != nil {
		return models.TracfinAIRun{}, err
	}
	return SqlToModel(ctx, exec, NewQueryBuilder().Select("r.content", "r.revision").From("tracfin_ai_runs AS r").Join("cases AS c ON c.id=r.case_id AND c.org_id=r.org_id").Where("r.org_id = ?", orgID).Where("r.case_id = ?", caseID).Where("r.id = ?", id), func(row dbTracfinAIRun) (models.TracfinAIRun, error) {
		var r models.TracfinAIRun
		err := json.Unmarshal(row.Content, &r)
		r.Revision = row.Revision
		return r, err
	})
}
func (repo *MarbleDbRepository) SaveTracfinAIRun(ctx context.Context, exec Executor, r models.TracfinAIRun, expected int64) error {
	if err := validateTracfinDraftScope(exec, r.OrgID, r.CaseID); err != nil {
		return err
	}
	if expected < 1 || r.ID == "" {
		return models.BadParameterError
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return errors.Wrap(err, "encode TRACFIN run")
	}
	return execTracfinWrite(ctx, exec, NewQueryBuilder().Update("tracfin_ai_runs").Set("content", raw).Set("status", r.Status).Set("revision", expected+1).Where("id = ?", r.ID).Where("org_id = ?", r.OrgID).Where("case_id = ?", r.CaseID).Where("revision = ?", expected).Where("EXISTS (SELECT 1 FROM cases WHERE cases.id=tracfin_ai_runs.case_id AND cases.org_id=tracfin_ai_runs.org_id)"))
}

// Manual edits release the active slot immediately; late worker results lose their CAS.
func (repo *MarbleDbRepository) InvalidateTracfinAIActive(ctx context.Context, exec Executor, orgID, caseID string) error {
	if err := validateTracfinDraftScope(exec, orgID, caseID); err != nil {
		return err
	}
	return ExecBuilder(ctx, exec, NewQueryBuilder().Update("tracfin_ai_runs").Set("status", "stale").Set("content", squirrel.Expr("jsonb_set(content, '{status}', '\"stale\"'::jsonb)")).Set("revision", squirrel.Expr("revision+1")).Where("org_id = ?", orgID).Where("case_id = ?", caseID).Where("status IN ('queued','running')"))
}

type dbTracfinAIRemark struct {
	ID        string    `db:"id"`
	Section   string    `db:"section"`
	Text      string    `db:"text"`
	ActorID   string    `db:"actor_id"`
	CreatedAt time.Time `db:"created_at"`
}

// ListTracfinAIRemarks returns every analyst remark made on the case's runs, oldest first.
func (repo *MarbleDbRepository) ListTracfinAIRemarks(ctx context.Context, exec Executor, orgID, caseID string) ([]models.TracfinAIRemark, error) {
	if err := validateTracfinDraftScope(exec, orgID, caseID); err != nil {
		return nil, err
	}
	return SqlToListOfModels(ctx, exec, NewQueryBuilder().
		Select("r.id::text AS id", "coalesce(r.content->>'feedback_section', substring(r.content->>'rework_reason' from 'requested section was ([a-z]+)'), r.content->>'from_section', '') AS section", "r.content->>'feedback' AS text", "r.actor_id::text AS actor_id", "r.created_at").
		From("tracfin_ai_runs AS r").Join("cases AS c ON c.id=r.case_id AND c.org_id=r.org_id").
		Where("r.org_id = ?", orgID).Where("r.case_id = ?", caseID).
		Where("coalesce(r.content->>'feedback', '') <> ''").
		Where("r.status <> 'refused'").
		OrderBy("r.created_at ASC"),
		func(row dbTracfinAIRemark) (models.TracfinAIRemark, error) {
			return models.TracfinAIRemark{RunID: row.ID, Section: row.Section, Text: row.Text, ActorID: row.ActorID, CreatedAt: row.CreatedAt}, nil
		})
}

// ListTracfinAIRuns returns the case's most recent runs, newest first.
func (repo *MarbleDbRepository) ListTracfinAIRuns(ctx context.Context, exec Executor, orgID, caseID string, limit int) ([]models.TracfinAIRun, error) {
	if err := validateTracfinDraftScope(exec, orgID, caseID); err != nil {
		return nil, err
	}
	return SqlToListOfModels(ctx, exec, NewQueryBuilder().Select("r.content", "r.revision").From("tracfin_ai_runs AS r").
		Join("cases AS c ON c.id=r.case_id AND c.org_id=r.org_id").
		Where("r.org_id = ?", orgID).Where("r.case_id = ?", caseID).
		OrderBy("r.created_at DESC").Limit(uint64(limit)),
		func(row dbTracfinAIRun) (models.TracfinAIRun, error) {
			var r models.TracfinAIRun
			err := json.Unmarshal(row.Content, &r)
			r.Revision = row.Revision
			return r, err
		})
}
