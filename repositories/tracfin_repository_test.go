package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/checkmarble/marble-backend/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type tracfinExecutor struct {
	pgxmock.PgxPoolIface
	sql    string
	args   []any
	schema models.DatabaseSchemaType
}

func newTracfinExecutor(t *testing.T) *tracfinExecutor {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.ExpectationsWereMet()); pool.Close() })
	return &tracfinExecutor{PgxPoolIface: pool, schema: models.DATABASE_SCHEMA_TYPE_MARBLE}
}
func (e *tracfinExecutor) DatabaseSchema() models.DatabaseSchema {
	return models.DatabaseSchema{SchemaType: e.schema}
}
func (e *tracfinExecutor) Cache(context.Context) *RedisExecutor       { return nil }
func (e *tracfinExecutor) Begin(context.Context) (Transaction, error) { panic("unexpected Begin") }
func (e *tracfinExecutor) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	e.sql, e.args = sql, args
	return e.PgxPoolIface.Exec(ctx, sql, args...)
}
func (e *tracfinExecutor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	e.sql, e.args = sql, args
	return e.PgxPoolIface.Query(ctx, sql, args...)
}

func TestTracfinDraftCreateDoesNotOverwrite(t *testing.T) {
	e := newTracfinExecutor(t)
	r := &MarbleDbRepository{}
	d := models.TracfinDraft{OrgID: "org", CaseID: "case", UpdatedBy: "author"}
	sql := `INSERT INTO tracfin_drafts .*SELECT .*FROM cases WHERE id = .* AND org_id = .* ON CONFLICT \(case_id\) DO NOTHING`
	e.ExpectExec(sql).WithArgs(pgxmock.AnyArg(), "author", "case", "org").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	require.NoError(t, r.CreateTracfinDraft(context.Background(), e, "org", "case", d))
	require.Contains(t, e.args, "org")
	require.Contains(t, e.args, "case")
	require.Contains(t, e.args, "author")
	require.NotContains(t, e.sql, "last_export")
	e.ExpectExec(sql).WithArgs(pgxmock.AnyArg(), "author", "case", "org").WillReturnResult(pgxmock.NewResult("INSERT", 0))
	require.ErrorIs(t, r.CreateTracfinDraft(context.Background(), e, "org", "case", d), models.ConflictError)
}
func TestTracfinConditionalMutations(t *testing.T) {
	for _, n := range []int64{0, 1} {
		for _, kind := range []string{"configuration", "draft", "export_first", "export_replace"} {
			t.Run(kind+string(rune('0'+n)), func(t *testing.T) {
				e := newTracfinExecutor(t)
				r := &MarbleDbRepository{}
				args := []any{pgxmock.AnyArg(), "", "org", "case", int64(7)}
				if kind == "configuration" {
					args = []any{pgxmock.AnyArg(), pgxmock.AnyArg(), "", "", "author", "org", int64(7)}
				}
				if kind == "export_first" {
					args = []any{pgxmock.AnyArg(), "org", "case", int64(7), nil}
				}
				if kind == "export_replace" {
					args = []any{pgxmock.AnyArg(), "org", "case", int64(7), "previous"}
				}
				e.ExpectExec(`UPDATE tracfin_`).WithArgs(args...).WillReturnResult(pgxmock.NewResult("UPDATE", n))
				var err error
				switch kind {
				case "configuration":
					err = r.SaveTracfinConfiguration(context.Background(), e, "org", models.TracfinConfiguration{OrgID: "org", UpdatedBy: "author"}, 7)
				case "draft":
					err = r.SaveTracfinDraft(context.Background(), e, "org", "case", models.TracfinDraft{OrgID: "org", CaseID: "case", LastExport: &models.TracfinExport{ID: "ignore"}}, 7)
					require.NotContains(t, e.sql, "last_export")
					require.Contains(t, e.sql, "revision = revision + 1")
				default:
					var prev *string
					if kind == "export_replace" {
						s := "previous"
						prev = &s
					}
					err = r.SaveTracfinExport(context.Background(), e, "org", "case", models.TracfinExport{ID: "new", DraftRevision: 7}, 7, prev)
					require.Contains(t, e.sql, "last_export->>'id' IS NOT DISTINCT FROM")
					if prev == nil {
						require.Contains(t, e.args, nil)
					} else {
						require.Contains(t, e.args, "previous")
					}
					require.NotContains(t, e.sql, "SET content")
					require.NotContains(t, e.sql, "revision = revision + 1")
					require.NotContains(t, e.sql, "updated_at =")
					require.NotContains(t, e.sql, "updated_by =")
				}
				require.Contains(t, e.sql, "revision =")
				require.Contains(t, e.sql, "org_id =")
				require.Contains(t, e.args, int64(7))
				require.Contains(t, e.args, "org")
				if kind != "configuration" {
					require.Contains(t, e.sql, "case_id =")
					require.Contains(t, e.sql, "EXISTS (SELECT 1 FROM cases")
					require.Contains(t, e.args, "case")
				}
				if n == 0 {
					require.ErrorIs(t, err, models.ConflictError)
				} else {
					require.NoError(t, err)
				}
			})
		}
	}
}
func TestTracfinConfigurationCreateExpectedZero(t *testing.T) {
	e := newTracfinExecutor(t)
	r := &MarbleDbRepository{}
	c := models.TracfinConfiguration{OrgID: "org", ProfileVersion: "version", ProfileHash: "hash", UpdatedBy: "author"}
	e.ExpectExec(`INSERT INTO tracfin_configuration .*ON CONFLICT \(org_id\) DO NOTHING`).WithArgs("org", pgxmock.AnyArg(), pgxmock.AnyArg(), "version", "hash", 1, "author").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	require.NoError(t, r.SaveTracfinConfiguration(context.Background(), e, "org", c, 0))
	require.Contains(t, e.args, "version")
	require.Contains(t, e.args, "hash")
	e.ExpectExec(`INSERT INTO tracfin_configuration`).WithArgs("org", pgxmock.AnyArg(), pgxmock.AnyArg(), "version", "hash", 1, "author").WillReturnResult(pgxmock.NewResult("INSERT", 0))
	require.ErrorIs(t, r.SaveTracfinConfiguration(context.Background(), e, "org", c, 0), models.ConflictError)
}

var tracfinDraftColumns = []string{"case_id", "org_id", "content", "revision", "last_export", "created_at", "updated_at", "updated_by"}
var tracfinConfigColumns = []string{"org_id", "settings", "mappings", "profile_version", "profile_hash", "revision", "created_at", "updated_at", "updated_by"}

func TestTracfinReadNotFoundAndIsolation(t *testing.T) {
	e := newTracfinExecutor(t)
	r := &MarbleDbRepository{}
	e.ExpectQuery(`SELECT .* FROM tracfin_configuration WHERE org_id = \$1`).WithArgs("org").WillReturnRows(pgxmock.NewRows(tracfinConfigColumns))
	_, err := r.GetTracfinConfiguration(context.Background(), e, "org")
	require.ErrorIs(t, err, models.NotFoundError)
	e.ExpectQuery(`SELECT .* FROM tracfin_drafts .*WHERE .*org_id = .* AND .*case_id =`).WithArgs("org", "case").WillReturnRows(pgxmock.NewRows(tracfinDraftColumns))
	_, err = r.GetTracfinDraft(context.Background(), e, "org", "case")
	require.ErrorIs(t, err, models.NotFoundError)
	require.Contains(t, e.sql, "cases")
}
func TestTracfinJSONRoundTrip(t *testing.T) {
	// Catch scalar normalization, lost scalar-attribute ownership and accidental export embedding.
	e := newTracfinExecutor(t)
	r := &MarbleDbRepository{}
	now := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	content := `{"document":{"root":{"id":"root","catalog_id":"/ds","fields":{"flag":[{"id":"flag","kind":"boolean","boolean":false}],"amount":[{"id":"money","kind":"decimal","decimal":"9007199254740993.0100"}],"zero":[{"id":"zero","kind":"decimal","decimal":"0"}],"attr":[{"id":"attr","parent_value_id":"nationality","kind":"text","text":"N"}]}}},"configuration":{"org_id":"org","profile_version":"frozen","profile_hash":"digest","revision":3,"settings":{"root":{"id":"settings"}},"mappings":[{"id":"map","target_field":"amount","source":{"kind":"decision_snapshot","field":"amount"}}]}}`
	e.ExpectQuery(`SELECT`).WithArgs("org", "case").WillReturnRows(pgxmock.NewRows(tracfinDraftColumns).AddRow("case", "org", []byte(content), int64(4), []byte(`{"id":"old","draft_revision":1,"blob_key":"immutable"}`), now, now, "author"))
	d, err := r.GetTracfinDraft(context.Background(), e, "org", "case")
	require.NoError(t, err)
	f := d.Document.Root.Fields
	require.NotContains(t, f, "absent")
	require.False(t, *f["flag"][0].Boolean)
	require.Equal(t, models.TracfinDecimal("9007199254740993.0100"), *f["amount"][0].Decimal)
	require.Equal(t, models.TracfinDecimal("0"), *f["zero"][0].Decimal)
	require.Equal(t, "nationality", f["attr"][0].ParentValueID)
	require.Equal(t, "old", d.LastExport.ID)
	require.Equal(t, int64(1), d.LastExport.DraftRevision)
	require.Equal(t, "frozen", d.Configuration.ProfileVersion)
	require.Equal(t, "digest", d.Configuration.ProfileHash)
	require.Equal(t, "map", d.Configuration.Mappings[0].ID)
	require.Equal(t, now, d.CreatedAt)
	require.Equal(t, "author", d.UpdatedBy)
	e.ExpectExec(`UPDATE`).WithArgs(pgxmock.AnyArg(), "author", "org", "case", int64(4)).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, r.SaveTracfinDraft(context.Background(), e, "org", "case", d, 4))
	var saved map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(e.args[0].([]byte), &saved))
	require.NotContains(t, saved, "last_export")
	require.NotContains(t, saved, "org_id")
	var doc models.TracfinDocument
	require.NoError(t, json.Unmarshal(saved["document"], &doc))
	require.Equal(t, d.Document, doc)
}
func TestTracfinConfigurationReadPreservesProfile(t *testing.T) {
	e := newTracfinExecutor(t)
	now := time.Now().UTC()
	e.ExpectQuery(`SELECT`).WithArgs("org").WillReturnRows(pgxmock.NewRows(tracfinConfigColumns).AddRow("org", []byte(`{"root":{"id":"settings"}}`), []byte(`[]`), "version", "hash", int64(8), now, now, "author"))
	c, err := (&MarbleDbRepository{}).GetTracfinConfiguration(context.Background(), e, "org")
	require.NoError(t, err)
	require.Equal(t, "version", c.ProfileVersion)
	require.Equal(t, "hash", c.ProfileHash)
	require.Equal(t, int64(8), c.Revision)
}
func TestTracfinRejectsInvalidScopeAndRevision(t *testing.T) {
	e := newTracfinExecutor(t)
	r := &MarbleDbRepository{}
	ctx := context.Background()
	for _, run := range []func() error{
		func() error { _, err := r.GetTracfinConfiguration(ctx, e, ""); return err },
		func() error { _, err := r.GetTracfinDraft(ctx, e, "org", ""); return err },
		func() error {
			return r.CreateTracfinDraft(ctx, e, "org", "case", models.TracfinDraft{OrgID: "other", CaseID: "case"})
		},
		func() error {
			return r.SaveTracfinDraft(ctx, e, "org", "case", models.TracfinDraft{OrgID: "org", CaseID: "case"}, 0)
		},
		func() error {
			return r.SaveTracfinConfiguration(ctx, e, "org", models.TracfinConfiguration{OrgID: "other"}, 0)
		},
		func() error {
			return r.SaveTracfinConfiguration(ctx, e, "org", models.TracfinConfiguration{OrgID: "org"}, -1)
		},
		func() error {
			return r.SaveTracfinExport(ctx, e, "org", "case", models.TracfinExport{ID: "new", DraftRevision: 3}, 4, nil)
		},
	} {
		require.ErrorIs(t, run(), models.BadParameterError)
	}
	e.schema = models.DATABASE_SCHEMA_TYPE_CLIENT
	_, err := r.GetTracfinDraft(ctx, e, "org", "case")
	require.Error(t, err)
}
func TestTracfinStorageAndJSONErrors(t *testing.T) {
	e := newTracfinExecutor(t)
	r := &MarbleDbRepository{}
	failure := errors.New("storage unavailable")
	e.ExpectExec(`UPDATE tracfin_drafts`).WithArgs(pgxmock.AnyArg(), "", "org", "case", int64(1)).WillReturnError(failure)
	require.ErrorIs(t, r.SaveTracfinDraft(context.Background(), e, "org", "case", models.TracfinDraft{OrgID: "org", CaseID: "case"}, 1), failure)
	e.ExpectQuery(`SELECT`).WithArgs("org", "case").WillReturnRows(pgxmock.NewRows(tracfinDraftColumns).AddRow("case", "org", []byte(`{"document":false}`), int64(1), nil, time.Now(), time.Now(), "author"))
	_, err := r.GetTracfinDraft(context.Background(), e, "org", "case")
	require.Error(t, err)
}
