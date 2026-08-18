package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

const (
	graphPkeyName      = "_graph_pkey"
	graphLookupName    = "idx_graph_lookup"
	graphUpdatedAtName = "idx_graph_updated_at"
	graphStatsName     = "_graph_stats"

	graphBuildTable = "_graph_build"

	// The live table is renamed here rather than dropped at swap time. Keeping it lets the rows the
	// incremental writer added during the build be carried over *after* the swap instead of before it,
	// which is what keeps the exclusive lock off the table walks read. See ReconcileGraphFromOld.
	graphOldTable = "_graph_old"

	// A walk holds the live table open while it reads, and the swap needs an exclusive lock on it.
	// Rather than queue behind a long walk — and make every later reader queue behind the swap —
	// give up and let the next run of the job try again.
	//
	// The swap is two catalog renames, so this bounds the wait to acquire and the hold is microseconds.
	// Nothing slower may be put under this lock: graph requests time out at DEFAULT_TIMEOUT_SECOND
	// (5s by default) and the middleware cancels the request context, so a walk that waits longer than
	// that does not come back slow, it comes back as an HTTP 408.
	graphSwapLockTimeout = "5s"

	// How far back the replay watermark is pulled, which has to exceed the longest a graph row can sit
	// stamped but not yet committed. Since the writer stamps clock_timestamp() as its last act before
	// the ingestion transaction commits, that is milliseconds — this is four orders of magnitude of
	// headroom, and being early costs only a wider idempotent replay. See GraphReplayWatermark.
	graphReplayWatermarkMargin = "1 minute"
)

type GraphBuilderRepository interface {
	CreateGraphBuildTable(ctx context.Context, exec Executor) error
	PopulateGraphBuildTable(ctx context.Context, exec Executor, recordType string, fields []models.Field) (int64, error)
	IndexGraphBuildTable(ctx context.Context, exec Executor) error
	ReplayGraphRows(ctx context.Context, exec Executor, since time.Time) (int64, error)
	AnalyzeGraphBuildTable(ctx context.Context, exec Executor) error
	SwapGraphTable(ctx context.Context, tx Transaction) error
	ReconcileGraphFromOld(ctx context.Context, tx Transaction, since time.Time) (int64, error)
	DropGraphBuildTable(ctx context.Context, exec Executor) error
	GraphReplayWatermark(ctx context.Context, exec Executor) (time.Time, error)
}

func (repo MarbleDbRepository) CreateGraphBuildTable(ctx context.Context, exec Executor) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	if err := repo.DropGraphBuildTable(ctx, exec); err != nil {
		return err
	}

	// A run that died between the swap and the reconcile leaves the previous generation renamed aside.
	// Nothing reads it, and the rows it still held that the live table lacks are regenerated from source
	// by the populate below, so it is simply cleared.
	if err := repo.dropTable(ctx, exec, graphOldTable); err != nil {
		return err
	}

	// Unlogged: the whole table is derived from the ingested data and rebuilt on a schedule, so
	// paying WAL for it would buy only the ability to recover something a later run reproduces
	// anyway. The trade is that a crash leaves the table empty rather than stale, and a walk over
	// an empty table returns its start node alone until the next build.
	sql := fmt.Sprintf(`create unlogged table %s (
		record_type varchar(64) not null,
		record_id text not null,
		field_name varchar(128) not null,
		field_value text not null,
		updated_at timestamp with time zone not null
	)`, pgIdentifierWithSchema(exec, graphBuildTable))

	if _, err := exec.Exec(ctx, sql); err != nil {
		return errors.Wrap(err, "error while creating the graph build table")
	}

	return nil
}

func (repo MarbleDbRepository) PopulateGraphBuildTable(
	ctx context.Context,
	exec Executor,
	recordType string,
	fields []models.Field,
) (int64, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return 0, err
	}
	if len(fields) == 0 {
		return 0, nil
	}

	// One row per participating field of a record, produced by unpivoting the columns through a
	// lateral VALUES list. This reads the source table once, where one statement per field would
	// read it once per field.
	sql := fmt.Sprintf(`
		insert into %s (record_type, record_id, field_name, field_value, updated_at)
		select %s, t.object_id, v.field_name, v.field_value, now()
		from %s t
		cross join lateral (values %s) as v(field_name, field_value)
		where t.valid_until = 'infinity'
			and v.field_value is not null
			and v.field_value <> ''`,
		pgIdentifierWithSchema(exec, graphBuildTable),
		pgClientDataIdentifierString(recordType),
		pgIdentifierWithSchema(exec, recordType),
		graphFieldUnpivot(fields))

	tag, err := exec.Exec(ctx, sql)
	if err != nil {
		return 0, errors.Wrapf(err, "error while populating the graph build table from %q", recordType)
	}

	return tag.RowsAffected(), nil
}

func (repo MarbleDbRepository) IndexGraphBuildTable(ctx context.Context, exec Executor) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	buildTable := pgIdentifierWithSchema(exec, graphBuildTable)

	// Index/constraint/statistics names are schema-scoped, not table-scoped, so they survive the
	// swap's table rename under whatever name they're given here. Suffixing each with a nonce
	// means this cycle's names can never collide with a still-live previous cycle's, so nothing
	// needs renaming after the swap.
	nonce := pure_utils.NewId().String()

	statements := []string{
		// The primary key doubles as the index for reading a record's fields, which is the walk's
		// hydration query.
		fmt.Sprintf("alter table %s add constraint %s primary key (record_type, record_id, field_name)",
			buildTable, pgx.Identifier.Sanitize([]string{fmt.Sprintf("%s_%s", graphPkeyName, nonce)})),

		// The walk's other query shape: find every record carrying a value on a field.
		fmt.Sprintf("create index %s on %s (record_type, field_name, field_value)",
			pgx.Identifier.Sanitize([]string{fmt.Sprintf("%s_%s", graphLookupName, nonce)}), buildTable),

		// Index used for ReplayGraphRows, which reads the rows the incremental writer added while
		// this build was running. BRIN because updated_at is near-monotonic and the index is on
		// the hot path of every incremental upsert.
		fmt.Sprintf("create index %s on %s using brin (updated_at)",
			pgx.Identifier.Sanitize([]string{fmt.Sprintf("%s_%s", graphUpdatedAtName, nonce)}), buildTable),

		// These three columns are heavily correlated — every row with field_name 'ip' is also a
		// row with record_type 'logins' — and the walk sizes a hypernode by asking the planner to
		// EXPLAIN a lookup keyed on all three at once. Estimating them independently multiplies
		// their selectivities and lands orders of magnitude low.
		fmt.Sprintf(
			"create statistics %s (ndistinct, dependencies, mcv) on record_type, field_name, field_value from %s",
			pgIdentifierWithSchema(exec, fmt.Sprintf("%s_%s", graphStatsName, nonce)), buildTable),
	}

	for _, sql := range statements {
		if _, err := exec.Exec(ctx, sql); err != nil {
			return errors.Wrap(err, "error while indexing the graph build table")
		}
	}

	return nil
}

// AnalyzeGraphBuildTable is separate from indexing only so the replay can run between the two: a
// table this young has no statistics at all, so without this the walk's hypernode estimate would be
// the planner's no-stats default and every count it reports would be meaningless. Analyzing before
// the swap rather than after means the live table is never briefly without statistics.
func (repo MarbleDbRepository) AnalyzeGraphBuildTable(ctx context.Context, exec Executor) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := fmt.Sprintf("analyze %s", pgIdentifierWithSchema(exec, graphBuildTable))
	if _, err := exec.Exec(ctx, sql); err != nil {
		return errors.Wrap(err, "error while analyzing the graph build table")
	}

	return nil
}

// ReplayGraphRows carries forward the rows the incremental ingestion writer put in the live table
// after this build read the source tables, so the table about to go live is not missing every record
// ingested since the build started. It runs before the swap and takes no lock the walk cares about,
// so it may take as long as it needs; whatever it still misses the reconcile afterwards picks up.
//
// One thing it cannot carry forward is a retraction: a field emptied incrementally mid-build left
// no row to replay, while the build's own snapshot still holds the old value. Such a value survives
// until the next build.
func (repo MarbleDbRepository) ReplayGraphRows(ctx context.Context, exec Executor, since time.Time) (int64, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return 0, err
	}

	// The first build of an organization has no live table to replay from.
	exists, err := repo.tableExists(ctx, exec, graphTable)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}

	return repo.replayGraphRows(ctx, exec, graphTable, graphBuildTable, since)
}

// replayGraphRows copies the rows stamped at or after `since` from one generation of the adjacency
// table into another. It runs in both directions: from the live table into the one being built, and
// after the swap from the previous generation into the new live one.
func (repo MarbleDbRepository) replayGraphRows(ctx context.Context, exec Executor, from, into string, since time.Time) (int64, error) {
	sql := fmt.Sprintf(`
		insert into %s as g (record_type, record_id, field_name, field_value, updated_at)
		select record_type, record_id, field_name, field_value, updated_at
		from %s
		where updated_at >= $1
		on conflict (record_type, record_id, field_name) do update set
			field_value = excluded.field_value,
			updated_at = excluded.updated_at
			where g.field_value <> excluded.field_value`,
		pgIdentifierWithSchema(exec, into),
		pgIdentifierWithSchema(exec, from))

	tag, err := exec.Exec(ctx, sql, since)
	if err != nil {
		return 0, errors.Wrapf(err, "error while replaying graph rows from %q into %q", from, into)
	}

	return tag.RowsAffected(), nil
}

// GraphReplayWatermark returns the timestamp the replay should catch up from: the clock, pulled back
// by a margin. The clock is read from the client database, not from the application: the stamps come from that
// same clock, so there is no skew term for the margin to absorb as well.
func (repo MarbleDbRepository) GraphReplayWatermark(ctx context.Context, exec Executor) (time.Time, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return time.Time{}, err
	}

	var watermark time.Time
	if err := exec.QueryRow(ctx, "select now() - $1::interval",
		graphReplayWatermarkMargin).Scan(&watermark); err != nil {
		return time.Time{}, errors.Wrap(err, "error while computing the graph replay watermark")
	}

	return watermark, nil
}

func (repo MarbleDbRepository) tableExists(ctx context.Context, exec Executor, table string) (bool, error) {
	sql := `select exists(select 1 from information_schema.tables
		where table_name = $1 and table_schema = $2)`

	var exists bool

	if err := exec.QueryRow(ctx, sql, table, exec.DatabaseSchema().Schema).Scan(&exists); err != nil {
		return false, errors.Wrapf(err, "error while checking whether %q exists", table)
	}

	return exists, nil
}

func (repo MarbleDbRepository) dropTable(ctx context.Context, exec Executor, table string) error {
	sql := fmt.Sprintf("drop table if exists %s", pgIdentifierWithSchema(exec, table))

	if _, err := exec.Exec(ctx, sql); err != nil {
		return errors.Wrapf(err, "error while dropping %q", table)
	}

	return nil
}

// SwapGraphTable makes the table just built the live one, moving the previous generation aside so
// we can replay missing rows.
func (repo MarbleDbRepository) SwapGraphTable(ctx context.Context, tx Transaction) error {
	if err := validateClientDbExecutor(tx); err != nil {
		return err
	}

	// LOCAL, so it lasts only for this transaction, and set first so it covers the renames below.
	if _, err := tx.Exec(ctx,
		fmt.Sprintf("set local lock_timeout = '%s'", graphSwapLockTimeout)); err != nil {
		return errors.Wrap(err, "error while bounding the graph swap's lock wait")
	}

	// The first build of an organization has no previous generation to move aside.
	exists, err := repo.tableExists(ctx, tx, graphTable)
	if err != nil {
		return err
	}

	statements := make([]string, 0, 2)

	if exists {
		statements = append(statements, fmt.Sprintf("alter table %s rename to %s",
			pgIdentifierWithSchema(tx, graphTable),
			pgx.Identifier.Sanitize([]string{graphOldTable})))
	}

	statements = append(statements, fmt.Sprintf("alter table %s rename to %s",
		pgIdentifierWithSchema(tx, graphBuildTable),
		pgx.Identifier.Sanitize([]string{graphTable})))

	for _, sql := range statements {
		if _, err := tx.Exec(ctx, sql); err != nil {
			return errors.Wrap(err, "error while swapping in the graph table")
		}
	}

	return nil
}

// ReconcileGraphFromOld finishes the swap: it carries the rows the previous generation still holds
// that the new live table lacks, then discards it. Runs in its own transaction, after the swap.
//
// The lock here is on the discarded table, not the live one, which is the whole point — no walk reads
// `_graph_old` by name, so however long this holds it costs readers nothing, and the rows it writes
// into the live table take only ROW EXCLUSIVE, which a reader's ACCESS SHARE does not conflict with.
//
// It is also what makes a single pass provably complete, so nothing here has to converge or iterate.
// A rename does not invalidate an OID, so two kinds of straggler are still on the old table after the
// swap, and this lock waits for both:
//
//   - an ingestion transaction that resolved `_graph` before the rename keeps writing into it and
//     commits successfully — silently, where a DROP would have failed it into its savepoint. Rows
//     therefore keep arriving after it stops being live, which a bare snapshot would miss.
//   - a walk that planned against the old OID is still reading it. Harmless in itself, since that
//     table holds more data rather than less, but the DROP must not pull the relation out from under
//     it.
func (repo MarbleDbRepository) ReconcileGraphFromOld(ctx context.Context, tx Transaction, since time.Time) (int64, error) {
	if err := validateClientDbExecutor(tx); err != nil {
		return 0, err
	}

	exists, err := repo.tableExists(ctx, tx, graphOldTable)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf("lock table %s in access exclusive mode",
		pgIdentifierWithSchema(tx, graphOldTable))); err != nil {
		return 0, errors.Wrap(err, "error while locking the previous graph generation")
	}

	replayed, err := repo.replayGraphRows(ctx, tx, graphOldTable, graphTable, since)
	if err != nil {
		return 0, err
	}

	if err := repo.dropTable(ctx, tx, graphOldTable); err != nil {
		return 0, err
	}

	return replayed, nil
}

func (repo MarbleDbRepository) DropGraphBuildTable(ctx context.Context, exec Executor) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	return repo.dropTable(ctx, exec, graphBuildTable)
}

// graphFieldUnpivot renders the participating fields as the lateral VALUES list that turns one
// record's columns into one row per field. Both the full build and the incremental writer go through
// it.
func graphFieldUnpivot(fields []models.Field) string {
	values := make([]string, 0, len(fields))

	for _, field := range fields {
		values = append(values, fmt.Sprintf("(%s, %s)",
			pgClientDataIdentifierString(field.Name), graphFieldProjection(field)))
	}

	return strings.Join(values, ", ")
}

// graphFieldProjection renders a column as the text the adjacency table stores and matches on.
// This cast is the equality function of the whole graph — two records are related when their
// projections are byte-equal — so each type gets the representation that makes "equal" mean what
// someone declaring a relation over that field would expect.
func graphFieldProjection(field models.Field) string {
	column := "t." + pgx.Identifier.Sanitize([]string{field.Name})

	switch field.DataType {
	case models.Coords:
		// A plain cast yields hex EWKB, which is both longer than EWKT and unreadable — and a
		// connector node is named after the value it stands for. Both are exact: coordinates
		// match only when identical, never by proximity.
		return fmt.Sprintf("st_asewkt(%s)", column)
	case models.Timestamp:
		return fmt.Sprintf("(%s at time zone 'utc')::text", column)
	default:
		return column + "::text"
	}
}
