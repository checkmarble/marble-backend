package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/segmentio/analytics-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/worker_jobs"
	"github.com/checkmarble/marble-backend/utils"
)

// TestGraphIncrementalMatchesAFullBuild is the test that justifies maintaining the adjacency table
// from the ingestion path at all.
//
// Two records are related when their stored projections are byte-equal, so a writer that renders a
// value even slightly differently from the periodic build does not fail — it silently stops matching,
// and the graph quietly loses edges. Asserting the two writers produce the same rows for the same
// data is the only check that catches that, and it needs a real database: the projections are SQL
// casts, and whether `t."opened_at"::text` and `(t."opened_at" at time zone 'utc')::text` differ is a
// question only Postgres can answer.
func TestGraphIncrementalMatchesAFullBuild(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	ctx = utils.StoreSegmentClientInContext(ctx, analytics.New("dummy key"))

	creds, _, _ := setupOrgAndCreds(ctx, t, "test org for graph maintenance")
	orgId := creds.OrganizationId
	uc := generateUsecaseWithCreds(testUsecases, creds)

	builder := newTestGraphBuilder()

	// The organization has no adjacency table until the first build runs, so the incremental write has
	// nothing to write to. It must not fail the ingestion over that.
	ingestGraphObject(ctx, t, uc, orgId, "companies",
		`{"object_id": "comp-0", "updated_at": "2026-01-01T00:00:00Z", "name": "Before Any Build"}`)

	require.NoError(t, builder.Build(ctx, orgId), "first build")

	// Ingested before the build, so the build itself is what put it in.
	assert.Contains(t, readGraphRows(ctx, t, orgId),
		graphTestRow{"companies", "comp-0", "object_id", "comp-0"})

	// A company, an account belonging to it, and a transaction on that account: the data model links
	// transactions.account_id → accounts.object_id and accounts.company_id → companies.object_id, so
	// all three participate.
	ingestGraphObject(ctx, t, uc, orgId, "companies",
		`{"object_id": "comp-1", "updated_at": "2026-01-01T00:00:00Z", "name": "Acme"}`)
	ingestGraphObject(ctx, t, uc, orgId, "accounts",
		`{"object_id": "acc-1", "updated_at": "2026-01-01T00:00:00Z", "company_id": "comp-1", "name": "Acme Main", "balance": 12.5}`)
	ingestGraphObject(ctx, t, uc, orgId, "transactions",
		`{"object_id": "tx-1", "updated_at": "2026-01-01T00:00:00Z", "account_id": "acc-1", "amount": 30.0}`)

	// The point of the whole change: reachable with no build in between.
	afterIngestion := readGraphRows(ctx, t, orgId)
	assert.Contains(t, afterIngestion, graphTestRow{"accounts", "acc-1", "company_id", "comp-1"},
		"the edge from the new account to its company is available immediately")
	assert.Contains(t, afterIngestion, graphTestRow{"transactions", "tx-1", "account_id", "acc-1"})
	assert.Contains(t, afterIngestion, graphTestRow{"transactions", "tx-1", "object_id", "tx-1"})

	// Only the linked fields and object_id: a field nothing traverses would bloat the table for
	// nothing, and one the walk reads but the table lacks silently finds nothing.
	assert.NotContains(t, afterIngestion, graphTestRow{"accounts", "acc-1", "name", "Acme Main"})

	// And they are exactly the rows a build from scratch produces.
	require.NoError(t, builder.Build(ctx, orgId), "rebuild after incremental ingestion")
	assert.ElementsMatch(t, afterIngestion, readGraphRows(ctx, t, orgId),
		"the incremental writer and the build must agree on every row, byte for byte")
}

func TestGraphIncrementalRetractsAndUpdates(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	ctx = utils.StoreSegmentClientInContext(ctx, analytics.New("dummy key"))

	creds, _, _ := setupOrgAndCreds(ctx, t, "test org for graph retraction")
	orgId := creds.OrganizationId
	uc := generateUsecaseWithCreds(testUsecases, creds)

	builder := newTestGraphBuilder()
	require.NoError(t, builder.Build(ctx, orgId), "first build")

	ingestGraphObject(ctx, t, uc, orgId, "accounts",
		`{"object_id": "acc-1", "updated_at": "2026-01-01T00:00:00Z", "company_id": "comp-1", "name": "Acme Main"}`)
	require.Contains(t, readGraphRows(ctx, t, orgId),
		graphTestRow{"accounts", "acc-1", "company_id", "comp-1"})

	// A newer version pointing at a different company: the old value must not linger, or the account
	// would appear to belong to both.
	ingestGraphObject(ctx, t, uc, orgId, "accounts",
		`{"object_id": "acc-1", "updated_at": "2026-01-02T00:00:00Z", "company_id": "comp-2", "name": "Acme Main"}`)

	moved := readGraphRows(ctx, t, orgId)
	assert.Contains(t, moved, graphTestRow{"accounts", "acc-1", "company_id", "comp-2"})
	assert.NotContains(t, moved, graphTestRow{"accounts", "acc-1", "company_id", "comp-1"},
		"the superseded value must not survive alongside the current one")

	// A newer version with no company at all. The adjacency table has no valid_until to mark a row
	// dead with and the upsert can only add or update, so this is what the retraction is for.
	ingestGraphObject(ctx, t, uc, orgId, "accounts",
		`{"object_id": "acc-1", "updated_at": "2026-01-03T00:00:00Z", "company_id": null, "name": "Acme Main"}`)

	retracted := readGraphRows(ctx, t, orgId)
	assert.NotContains(t, retracted, graphTestRow{"accounts", "acc-1", "company_id", "comp-2"},
		"a field the new version left empty must lose its row, not keep the old value")
	assert.Contains(t, retracted, graphTestRow{"accounts", "acc-1", "object_id", "acc-1"},
		"the record itself is still there")

	// Every step above must leave the table in the state a build would.
	require.NoError(t, builder.Build(ctx, orgId), "rebuild after updates and retraction")
	assert.ElementsMatch(t, retracted, readGraphRows(ctx, t, orgId))
}

// TestGraphIncrementalRetractsAnOmittedField covers the way a value is most likely to actually go
// away in production: not an explicit null, but a client that simply stops sending the field. A POST
// does not carry missing fields over from the previous version — only a PATCH does — so an omitted
// nullable field is ingested as NULL, and the row the previous version left in the adjacency table
// has nothing to overwrite it.
func TestGraphIncrementalRetractsAnOmittedField(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	ctx = utils.StoreSegmentClientInContext(ctx, analytics.New("dummy key"))

	creds, _, _ := setupOrgAndCreds(ctx, t, "test org for graph omitted field")
	orgId := creds.OrganizationId
	uc := generateUsecaseWithCreds(testUsecases, creds)

	builder := newTestGraphBuilder()
	require.NoError(t, builder.Build(ctx, orgId), "first build")

	ingestGraphObject(ctx, t, uc, orgId, "accounts",
		`{"object_id": "acc-1", "updated_at": "2026-01-01T00:00:00Z", "company_id": "comp-1"}`)
	require.Contains(t, readGraphRows(ctx, t, orgId),
		graphTestRow{"accounts", "acc-1", "company_id", "comp-1"})

	// Same record, newer version, company_id simply not mentioned.
	ingestGraphObject(ctx, t, uc, orgId, "accounts",
		`{"object_id": "acc-1", "updated_at": "2026-01-02T00:00:00Z", "name": "Acme Main"}`)

	rows := readGraphRows(ctx, t, orgId)
	assert.NotContains(t, rows, graphTestRow{"accounts", "acc-1", "company_id", "comp-1"},
		"an omitted nullable field is ingested as NULL, so its adjacency row is now stale")

	// The build reading the live row is the arbiter of what the table should hold: if it agrees the
	// field is gone, a row the incremental writer left behind would be an edge that does not exist.
	require.NoError(t, builder.Build(ctx, orgId), "rebuild after the omission")
	assert.ElementsMatch(t, rows, readGraphRows(ctx, t, orgId))
}

// TestGraphReconcileCarriesRowsIngestedMidBuild drives the build a step at a time so a record can be
// ingested at the one moment only the reconcile can rescue it: after the bulk catch-up has already run.
//
// This is the case the whole replay/reconcile machinery exists for. A build can take hours, and every
// record ingested during it lands in the live table the swap is about to retire — so without this, each
// build would silently discard a day's worth of incremental freshness. The other tests here ingest
// before the build, where the bulk pass picks everything up and the reconcile has nothing to do.
func TestGraphReconcileCarriesRowsIngestedMidBuild(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	ctx = utils.StoreSegmentClientInContext(ctx, analytics.New("dummy key"))

	creds, dataModel, _ := setupOrgAndCreds(ctx, t, "test org for graph mid-build ingestion")
	orgId := creds.OrganizationId
	uc := generateUsecaseWithCreds(testUsecases, creds)

	admin := generateUsecaseWithCredForMarbleAdmin(testUsecases)
	repo := admin.Repositories.MarbleDbRepository
	fields := models.GraphIndexedFields(dataModel, nil)

	// A live table has to exist for there to be anything to retire.
	require.NoError(t, newTestGraphBuilder().Build(ctx, orgId), "first build")

	clientExec, err := admin.NewExecutorFactory().NewClientDbExecutor(ctx, orgId)
	require.NoError(t, err)

	require.NoError(t, repo.CreateGraphBuildTable(ctx, clientExec))

	watermark, err := repo.GraphReplayWatermark(ctx, clientExec)
	require.NoError(t, err)

	for recordType, recordFields := range fields {
		_, err := repo.PopulateGraphBuildTable(ctx, clientExec, recordType, recordFields)
		require.NoError(t, err)
	}
	require.NoError(t, repo.IndexGraphBuildTable(ctx, clientExec))

	reconcileWatermark, err := repo.GraphReplayWatermark(ctx, clientExec)
	require.NoError(t, err)

	_, err = repo.ReplayGraphRows(ctx, clientExec, watermark)
	require.NoError(t, err)

	// The mid-build arrival. The bulk pass above has already run, so this row exists only in the live
	// table — the one the swap is about to rename aside.
	ingestGraphObject(ctx, t, uc, orgId, "accounts",
		`{"object_id": "acc-late", "updated_at": "2026-01-01T00:00:00Z", "company_id": "comp-late"}`)

	late := graphTestRow{"accounts", "acc-late", "company_id", "comp-late"}
	require.Contains(t, readGraphRows(ctx, t, orgId), late,
		"ingestion put it in the live table, which is the premise of the rest of this test")

	require.NoError(t, repo.AnalyzeGraphBuildTable(ctx, clientExec))

	require.NoError(t, admin.NewTransactionFactory().TransactionInOrgSchema(ctx, orgId,
		func(tx repositories.Transaction) error {
			return repo.SwapGraphTable(ctx, tx)
		}))

	// The window the rename-aside design accepts, asserted rather than assumed: the new table is live
	// and does not yet hold the tail. Incompleteness, not corruption, and no reader was ever blocked
	// for it.
	assert.NotContains(t, readGraphRows(ctx, t, orgId), late,
		"the freshly built table cannot know about a record ingested after it was populated")

	var replayed int64
	require.NoError(t, admin.NewTransactionFactory().TransactionInOrgSchema(ctx, orgId,
		func(tx repositories.Transaction) error {
			replayed, err = repo.ReconcileGraphFromOld(ctx, tx, reconcileWatermark)
			return err
		}))

	assert.Positive(t, replayed, "the reconcile is what carries the tail over, so it must have written")
	assert.Contains(t, readGraphRows(ctx, t, orgId), late,
		"a record ingested mid-build must survive the build that was running at the time")

	// And the previous generation is gone, so the next build starts clean.
	var oldExists bool
	require.NoError(t, clientExec.QueryRow(ctx,
		`select exists(select 1 from information_schema.tables
			where table_name = '_graph_old' and table_schema = $1)`,
		clientExec.DatabaseSchema().Schema).Scan(&oldExists))
	assert.False(t, oldExists, "the reconcile discards the generation it drained")
}

func newTestGraphBuilder() worker_jobs.GraphBuilder {
	admin := generateUsecaseWithCredForMarbleAdmin(testUsecases)

	return worker_jobs.NewGraphBuilder(
		admin.NewExecutorFactory(),
		admin.NewTransactionFactory(),
		admin.NewFeatureAccessReader(),
		admin.Repositories.MarbleDbRepository,
		admin.Repositories.MarbleDbRepository,
		admin.Repositories.MarbleDbRepository,
	)
}

func ingestGraphObject(
	ctx context.Context,
	t *testing.T,
	uc usecases.UsecasesWithCreds,
	orgId uuid.UUID,
	objectType string,
	payload string,
) {
	t.Helper()

	ingestion := uc.NewIngestionUseCase()
	_, err := ingestion.IngestObject(ctx, orgId, objectType, []byte(payload), models.IngestionOptions{})
	require.NoErrorf(t, err, "could not ingest %s %s", objectType, payload)
}

// graphTestRow is a row of the adjacency table, comparable so a whole table can be compared as a set.
// updated_at is left out on purpose: it is bookkeeping for the replay, and a build and an incremental
// write will never agree on it.
type graphTestRow struct {
	RecordType string
	RecordId   string
	FieldName  string
	FieldValue string
}

func readGraphRows(ctx context.Context, t *testing.T, orgId uuid.UUID) []graphTestRow {
	t.Helper()

	exec, err := testUsecases.NewExecutorFactory().NewClientDbExecutor(ctx, orgId)
	require.NoError(t, err)

	sql := fmt.Sprintf(
		"select record_type, record_id, field_name, field_value from %s",
		pgx.Identifier{exec.DatabaseSchema().Schema, "_graph"}.Sanitize())

	rows, err := exec.Query(ctx, sql)
	require.NoError(t, err)
	defer rows.Close()

	out := make([]graphTestRow, 0)
	for rows.Next() {
		var row graphTestRow
		require.NoError(t, rows.Scan(&row.RecordType, &row.RecordId, &row.FieldName, &row.FieldValue))
		out = append(out, row)
	}
	require.NoError(t, rows.Err())

	return out
}
