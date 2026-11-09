package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

// SarRepository persists the SAR pivot, reports, revisions, runs, memory and
// chat. Every query is scoped by organization; documents are JSONB.
type SarRepository interface {
	GetSarConfiguration(ctx context.Context, exec Executor, orgID string) (models.SarConfiguration, error)
	SaveSarConfiguration(ctx context.Context, exec Executor, c models.SarConfiguration, expected int64) error
	GetSarPivot(ctx context.Context, exec Executor, orgID, caseID string, forUpdate bool) (models.SarPivot, bool, error)
	SaveSarPivot(ctx context.Context, exec Executor, p models.SarPivot, expected int64) error
	ListSarReports(ctx context.Context, exec Executor, orgID, caseID string) ([]models.SarReport, error)
	GetSarReport(ctx context.Context, exec Executor, orgID, caseID, id string) (models.SarReport, error)
	SaveSarReport(ctx context.Context, exec Executor, r models.SarReport, expected int64) error
	AddSarRevision(ctx context.Context, exec Executor, r models.SarRevision, snapshot any) error
	ListSarRevisions(ctx context.Context, exec Executor, orgID, caseID, object string) ([]models.SarRevision, error)
	CreateSarRun(ctx context.Context, exec Executor, r models.SarRun) error
	GetSarRun(ctx context.Context, exec Executor, orgID, caseID, id string) (models.SarRun, error)
	SaveSarRun(ctx context.Context, exec Executor, r models.SarRun) error
	ListSarMemory(ctx context.Context, exec Executor, orgID, formatKey string, limit int) ([]models.SarMemoryEntry, error)
	UpsertSarMemory(ctx context.Context, exec Executor, e models.SarMemoryEntry) error
	AddSarChatMessage(ctx context.Context, exec Executor, m models.SarChatMessage) error
	ListSarChatMessages(ctx context.Context, exec Executor, orgID, reportID string) ([]models.SarChatMessage, error)
	UpdateSarChatMessage(ctx context.Context, exec Executor, m models.SarChatMessage) error
}

type dbSarContent struct {
	Content  json.RawMessage `db:"content"`
	Revision int64           `db:"revision"`
}

func sarScope(orgID string, ids ...string) error {
	if _, err := uuid.Parse(orgID); err != nil {
		return errors.Wrap(models.BadParameterError, "organization is required")
	}
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return errors.Wrap(models.BadParameterError, "invalid identifier")
		}
	}
	return nil
}

func sarExec(ctx context.Context, exec Executor, q squirrel.Sqlizer, single bool) error {
	sql, args, err := q.ToSql()
	if err != nil {
		return err
	}
	tag, err := exec.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if single && tag.RowsAffected() != 1 {
		return errors.Wrap(models.ConflictError, "SAR record changed since it was read")
	}
	return nil
}

func (repo *MarbleDbRepository) GetSarConfiguration(ctx context.Context, exec Executor, orgID string) (models.SarConfiguration, error) {
	if err := sarScope(orgID); err != nil {
		return models.SarConfiguration{}, err
	}
	rows, err := SqlToListOfModels(ctx, exec, NewQueryBuilder().Select("content", "revision").From("sar_configurations").Where("org_id = ?", orgID), func(r dbSarContent) (models.SarConfiguration, error) {
		var c models.SarConfiguration
		err := json.Unmarshal(r.Content, &c)
		c.Revision = r.Revision
		return c, err
	})
	if err != nil || len(rows) == 0 {
		return models.SarConfiguration{OrgID: orgID}, err
	}
	rows[0].OrgID = orgID
	return rows[0], nil
}

func (repo *MarbleDbRepository) SaveSarConfiguration(ctx context.Context, exec Executor, c models.SarConfiguration, expected int64) error {
	if err := sarScope(c.OrgID); err != nil {
		return err
	}
	c.Revision = expected + 1
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if expected == 0 {
		return sarExec(ctx, exec, NewQueryBuilder().Insert("sar_configurations").Columns("org_id", "revision", "content").Values(c.OrgID, 1, raw).Suffix("ON CONFLICT DO NOTHING"), true)
	}
	return sarExec(ctx, exec, NewQueryBuilder().Update("sar_configurations").Set("content", raw).Set("revision", expected+1).Set("updated_at", squirrel.Expr("now()")).Where("org_id = ?", c.OrgID).Where("revision = ?", expected), true)
}

func (repo *MarbleDbRepository) GetSarPivot(ctx context.Context, exec Executor, orgID, caseID string, forUpdate bool) (models.SarPivot, bool, error) {
	if err := sarScope(orgID, caseID); err != nil {
		return models.SarPivot{}, false, err
	}
	q := NewQueryBuilder().Select("content", "revision").From("sar_pivots").Where("org_id = ?", orgID).Where("case_id = ?", caseID)
	if forUpdate {
		q = q.Suffix("FOR UPDATE")
	}
	rows, err := SqlToListOfModels(ctx, exec, q, func(r dbSarContent) (models.SarPivot, error) {
		var p models.SarPivot
		err := json.Unmarshal(r.Content, &p)
		p.Revision = r.Revision
		return p, err
	})
	if err != nil || len(rows) == 0 {
		return models.SarPivot{CaseID: caseID, OrgID: orgID}, false, err
	}
	return rows[0], true, nil
}

// SaveSarPivot inserts revision 1 when expected is 0, else compares and swaps.
func (repo *MarbleDbRepository) SaveSarPivot(ctx context.Context, exec Executor, p models.SarPivot, expected int64) error {
	if err := sarScope(p.OrgID, p.CaseID); err != nil {
		return err
	}
	p.Revision = expected + 1
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if expected == 0 {
		return sarExec(ctx, exec, NewQueryBuilder().Insert("sar_pivots").Columns("case_id", "org_id", "revision", "content").
			Select(NewQueryBuilder().Select().Column("?::uuid, ?::uuid, 1, ?::jsonb", p.CaseID, p.OrgID, raw).From("cases").Where("id = ?", p.CaseID).Where("org_id = ?", p.OrgID)).Suffix("ON CONFLICT DO NOTHING"), true)
	}
	return sarExec(ctx, exec, NewQueryBuilder().Update("sar_pivots").Set("content", raw).Set("revision", expected+1).Set("updated_at", squirrel.Expr("now()")).
		Where("org_id = ?", p.OrgID).Where("case_id = ?", p.CaseID).Where("revision = ?", expected), true)
}

func (repo *MarbleDbRepository) ListSarReports(ctx context.Context, exec Executor, orgID, caseID string) ([]models.SarReport, error) {
	if err := sarScope(orgID, caseID); err != nil {
		return nil, err
	}
	return SqlToListOfModels(ctx, exec, NewQueryBuilder().Select("content", "revision").From("sar_reports").Where("org_id = ?", orgID).Where("case_id = ?", caseID).OrderBy("format_key"), adaptSarReport)
}

func adaptSarReport(r dbSarContent) (models.SarReport, error) {
	var rep models.SarReport
	err := json.Unmarshal(r.Content, &rep)
	rep.Revision = r.Revision
	return rep, err
}

func (repo *MarbleDbRepository) GetSarReport(ctx context.Context, exec Executor, orgID, caseID, id string) (models.SarReport, error) {
	if err := sarScope(orgID, caseID, id); err != nil {
		return models.SarReport{}, err
	}
	return SqlToModel(ctx, exec, NewQueryBuilder().Select("content", "revision").From("sar_reports").Where("org_id = ?", orgID).Where("case_id = ?", caseID).Where("id = ?", id), adaptSarReport)
}

func (repo *MarbleDbRepository) SaveSarReport(ctx context.Context, exec Executor, r models.SarReport, expected int64) error {
	if err := sarScope(r.OrgID, r.CaseID, r.ID); err != nil {
		return err
	}
	r.Revision = expected + 1
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if expected == 0 {
		return sarExec(ctx, exec, NewQueryBuilder().Insert("sar_reports").Columns("id", "case_id", "org_id", "format_key", "revision", "content").
			Select(NewQueryBuilder().Select().Column("?::uuid, ?::uuid, ?::uuid, ?::text, 1, ?::jsonb", r.ID, r.CaseID, r.OrgID, r.FormatKey, raw).From("cases").Where("id = ?", r.CaseID).Where("org_id = ?", r.OrgID)).Suffix("ON CONFLICT DO NOTHING"), true)
	}
	return sarExec(ctx, exec, NewQueryBuilder().Update("sar_reports").Set("content", raw).Set("revision", expected+1).Set("updated_at", squirrel.Expr("now()")).
		Where("org_id = ?", r.OrgID).Where("case_id = ?", r.CaseID).Where("id = ?", r.ID).Where("revision = ?", expected), true)
}

func (repo *MarbleDbRepository) AddSarRevision(ctx context.Context, exec Executor, r models.SarRevision, snapshot any) error {
	if err := sarScope(r.OrgID, r.CaseID); err != nil {
		return err
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	var patch any
	if len(r.Patch) > 0 {
		patch = []byte(r.Patch)
	}
	return sarExec(ctx, exec, NewQueryBuilder().Insert("sar_revisions").Columns("id", "org_id", "case_id", "object", "revision", "origin", "actor_id", "run_id", "chat_id", "summary", "patch", "snapshot").
		Values(uuid.NewString(), r.OrgID, r.CaseID, r.Object, r.Revision, r.Origin, r.ActorID, r.RunID, r.ChatID, r.Summary, patch, raw), true)
}

type dbSarRevision struct {
	ID        string          `db:"id"`
	OrgID     string          `db:"org_id"`
	CaseID    string          `db:"case_id"`
	Object    string          `db:"object"`
	Revision  int64           `db:"revision"`
	Origin    string          `db:"origin"`
	ActorID   string          `db:"actor_id"`
	RunID     string          `db:"run_id"`
	ChatID    string          `db:"chat_id"`
	Summary   string          `db:"summary"`
	Patch     json.RawMessage `db:"patch"`
	CreatedAt time.Time       `db:"created_at"`
}

func (repo *MarbleDbRepository) ListSarRevisions(ctx context.Context, exec Executor, orgID, caseID, object string) ([]models.SarRevision, error) {
	if err := sarScope(orgID, caseID); err != nil {
		return nil, err
	}
	q := NewQueryBuilder().Select("id", "org_id", "case_id", "object", "revision", "origin", "actor_id", "run_id", "chat_id", "summary", "patch", "created_at").From("sar_revisions").Where("org_id = ?", orgID).Where("case_id = ?", caseID).OrderBy("created_at DESC").Limit(500)
	if object != "" {
		q = q.Where("object = ?", object)
	}
	return SqlToListOfModels(ctx, exec, q, func(r dbSarRevision) (models.SarRevision, error) {
		return models.SarRevision{ID: r.ID, OrgID: r.OrgID, CaseID: r.CaseID, Object: r.Object, Revision: r.Revision, Origin: r.Origin, ActorID: r.ActorID, RunID: r.RunID, ChatID: r.ChatID, Summary: r.Summary, Patch: r.Patch, CreatedAt: r.CreatedAt}, nil
	})
}

func (repo *MarbleDbRepository) CreateSarRun(ctx context.Context, exec Executor, r models.SarRun) error {
	if err := sarScope(r.OrgID, r.CaseID, r.ID); err != nil {
		return err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	err = sarExec(ctx, exec, NewQueryBuilder().Insert("sar_runs").Columns("id", "org_id", "case_id", "status", "content").
		Select(NewQueryBuilder().Select().Column("?::uuid, ?::uuid, ?::uuid, ?::text, ?::jsonb", r.ID, r.OrgID, r.CaseID, r.Status, raw).From("cases").Where("id = ?", r.CaseID).Where("org_id = ?", r.OrgID)), true)
	if IsUniqueViolationError(err) {
		return errors.Wrap(models.ConflictError, "a SAR run is already active for this case")
	}
	return err
}

func (repo *MarbleDbRepository) GetSarRun(ctx context.Context, exec Executor, orgID, caseID, id string) (models.SarRun, error) {
	if err := sarScope(orgID, caseID, id); err != nil {
		return models.SarRun{}, err
	}
	return SqlToModel(ctx, exec, NewQueryBuilder().Select("content", "1 AS revision").From("sar_runs").Where("org_id = ?", orgID).Where("case_id = ?", caseID).Where("id = ?", id), func(r dbSarContent) (models.SarRun, error) {
		var run models.SarRun
		return run, json.Unmarshal(r.Content, &run)
	})
}

func (repo *MarbleDbRepository) SaveSarRun(ctx context.Context, exec Executor, r models.SarRun) error {
	if err := sarScope(r.OrgID, r.CaseID, r.ID); err != nil {
		return err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return sarExec(ctx, exec, NewQueryBuilder().Update("sar_runs").Set("content", raw).Set("status", r.Status).Set("updated_at", squirrel.Expr("now()")).Where("org_id = ?", r.OrgID).Where("case_id = ?", r.CaseID).Where("id = ?", r.ID), true)
}

type dbSarMemory struct {
	ID        string    `db:"id"`
	OrgID     string    `db:"org_id"`
	FormatKey string    `db:"format_key"`
	Code      string    `db:"code"`
	Path      string    `db:"path"`
	Cause     string    `db:"cause"`
	Fix       string    `db:"fix"`
	Frequency int       `db:"frequency"`
	Corrected int       `db:"corrected"`
	FirstSeen time.Time `db:"first_seen"`
	LastSeen  time.Time `db:"last_seen"`
}

func (repo *MarbleDbRepository) ListSarMemory(ctx context.Context, exec Executor, orgID, formatKey string, limit int) ([]models.SarMemoryEntry, error) {
	if err := sarScope(orgID); err != nil {
		return nil, err
	}
	q := NewQueryBuilder().Select("id", "org_id", "format_key", "code", "path", "cause", "fix", "frequency", "corrected", "first_seen", "last_seen").From("sar_format_memory").Where("org_id = ?", orgID).OrderBy("frequency DESC", "last_seen DESC")
	if formatKey != "" {
		q = q.Where("format_key = ?", formatKey)
	}
	if limit > 0 {
		q = q.Limit(uint64(limit))
	}
	return SqlToListOfModels(ctx, exec, q, func(r dbSarMemory) (models.SarMemoryEntry, error) {
		return models.SarMemoryEntry(r), nil
	})
}

// UpsertSarMemory deduplicates on (org, format, code, normalized path): the
// frequency grows, the latest cause is kept and a known fix is never erased.
func (repo *MarbleDbRepository) UpsertSarMemory(ctx context.Context, exec Executor, e models.SarMemoryEntry) error {
	if err := sarScope(e.OrgID); err != nil {
		return err
	}
	return sarExec(ctx, exec, NewQueryBuilder().Insert("sar_format_memory").Columns("id", "org_id", "format_key", "code", "path", "cause", "fix", "frequency", "corrected").
		Values(uuid.NewString(), e.OrgID, e.FormatKey, e.Code, e.Path, e.Cause, e.Fix, 1, e.Corrected).
		Suffix(`ON CONFLICT (org_id, format_key, code, path) DO UPDATE SET frequency = sar_format_memory.frequency + 1,
 corrected = sar_format_memory.corrected + EXCLUDED.corrected, cause = EXCLUDED.cause,
 fix = CASE WHEN EXCLUDED.fix <> '' THEN EXCLUDED.fix ELSE sar_format_memory.fix END, last_seen = now()`), false)
}

func (repo *MarbleDbRepository) AddSarChatMessage(ctx context.Context, exec Executor, m models.SarChatMessage) error {
	if err := sarScope(m.OrgID, m.CaseID, m.ReportID, m.ID); err != nil {
		return err
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return sarExec(ctx, exec, NewQueryBuilder().Insert("sar_chat_messages").Columns("id", "org_id", "case_id", "report_id", "content").Values(m.ID, m.OrgID, m.CaseID, m.ReportID, raw), true)
}

func (repo *MarbleDbRepository) UpdateSarChatMessage(ctx context.Context, exec Executor, m models.SarChatMessage) error {
	if err := sarScope(m.OrgID, m.ID); err != nil {
		return err
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return sarExec(ctx, exec, NewQueryBuilder().Update("sar_chat_messages").Set("content", raw).Where("org_id = ?", m.OrgID).Where("id = ?", m.ID), true)
}

func (repo *MarbleDbRepository) ListSarChatMessages(ctx context.Context, exec Executor, orgID, reportID string) ([]models.SarChatMessage, error) {
	if err := sarScope(orgID, reportID); err != nil {
		return nil, err
	}
	return SqlToListOfModels(ctx, exec, NewQueryBuilder().Select("content", "1 AS revision").From("sar_chat_messages").Where("org_id = ?", orgID).Where("report_id = ?", reportID).OrderBy("created_at"), func(r dbSarContent) (models.SarChatMessage, error) {
		var m models.SarChatMessage
		return m, json.Unmarshal(r.Content, &m)
	})
}
