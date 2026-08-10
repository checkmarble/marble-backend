package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
)

// The adjacency table is rebuilt from scratch rather than updated in place, so a build writes to
// a table of its own and swaps it in only once it is complete and indexed. Readers keep seeing
// the previous graph for the whole duration of a build.
const (
	graphBuildTable = "_graph_build"

	// Index names are schema-scoped, so the build table's own indexes have to be named apart from
	// the live ones and renamed during the swap: ALTER TABLE ... RENAME TO leaves the names of
	// constraints and indexes alone. Without this, the second build would collide with the first.
	graphPkeyName       = "_graph_pkey"
	graphLookupName     = "idx__graph_lookup"
	graphBuildPkey      = "_graph_build_pkey"
	graphBuildLookupIdx = "idx__graph_build_lookup"

	// A walk holds the live table open while it reads, and the swap needs an exclusive lock on it.
	// Rather than queue behind a long walk — and make every later reader queue behind the swap —
	// give up and let the next run of the job try again.
	graphSwapLockTimeout = "5s"
)

type GraphBuilderRepository interface {
	CreateGraphBuildTable(ctx context.Context, exec Executor) error
	PopulateGraphBuildTable(ctx context.Context, exec Executor, recordType string, fields []models.Field) (int64, error)
	IndexGraphBuildTable(ctx context.Context, exec Executor) error
	SwapGraphTable(ctx context.Context, tx Transaction) error
	DropGraphBuildTable(ctx context.Context, exec Executor) error
}

type GraphBuilderRepositoryPostgresql struct{}

func (repo GraphBuilderRepositoryPostgresql) CreateGraphBuildTable(ctx context.Context, exec Executor) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	if err := repo.DropGraphBuildTable(ctx, exec); err != nil {
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

func (repo GraphBuilderRepositoryPostgresql) PopulateGraphBuildTable(
	ctx context.Context, exec Executor, recordType string, fields []models.Field,
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
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		values = append(values, fmt.Sprintf("(%s, %s)",
			pgStringLiteral(field.Name), graphFieldProjection(field)))
	}

	sql := fmt.Sprintf(`
		insert into %s (record_type, record_id, field_name, field_value, updated_at)
		select %s, t.object_id, v.field_name, v.field_value, now()
		from %s t
		cross join lateral (values %s) as v(field_name, field_value)
		where t.valid_until = 'infinity'
			and v.field_value is not null
			and v.field_value <> ''`,
		pgIdentifierWithSchema(exec, graphBuildTable),
		pgStringLiteral(recordType),
		pgIdentifierWithSchema(exec, recordType),
		strings.Join(values, ", "))

	tag, err := exec.Exec(ctx, sql)
	if err != nil {
		return 0, errors.Wrapf(err, "error while populating the graph build table from %q", recordType)
	}

	return tag.RowsAffected(), nil
}

func (repo GraphBuilderRepositoryPostgresql) IndexGraphBuildTable(ctx context.Context, exec Executor) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	buildTable := pgIdentifierWithSchema(exec, graphBuildTable)

	// The primary key doubles as the index for reading a record's fields, which is the walk's
	// hydration query. It also asserts one live row per object_id: a violation here means the
	// source table's unique index on object_id is missing or was bypassed, which would make the
	// graph wrong in ways that are far harder to notice than a failed build.
	statements := []string{
		fmt.Sprintf("alter table %s add constraint %s primary key (record_type, record_id, field_name)",
			buildTable, pgx.Identifier.Sanitize([]string{graphBuildPkey})),
		// The walk's other query shape: find every record carrying a value on a field.
		fmt.Sprintf("create index %s on %s (record_type, field_name, field_value)",
			pgx.Identifier.Sanitize([]string{graphBuildLookupIdx}), buildTable),
	}

	for _, sql := range statements {
		if _, err := exec.Exec(ctx, sql); err != nil {
			return errors.Wrap(err, "error while indexing the graph build table")
		}
	}

	return nil
}

func (repo GraphBuilderRepositoryPostgresql) SwapGraphTable(ctx context.Context, tx Transaction) error {
	if err := validateClientDbExecutor(tx); err != nil {
		return err
	}

	statements := []string{
		// Bounded so a walk in flight delays the swap rather than blocking every later reader
		// behind it. LOCAL, so it lasts only for this transaction.
		fmt.Sprintf("set local lock_timeout = '%s'", graphSwapLockTimeout),
		fmt.Sprintf("drop table if exists %s", pgIdentifierWithSchema(tx, graphTable)),
		fmt.Sprintf("alter table %s rename to %s",
			pgIdentifierWithSchema(tx, graphBuildTable),
			pgx.Identifier.Sanitize([]string{graphTable})),

		// Renaming the table leaves these behind under their build-time names, and the next build
		// would then fail to create them.
		fmt.Sprintf("alter index %s rename to %s",
			pgIdentifierWithSchema(tx, graphBuildPkey),
			pgx.Identifier.Sanitize([]string{graphPkeyName})),
		fmt.Sprintf("alter index %s rename to %s",
			pgIdentifierWithSchema(tx, graphBuildLookupIdx),
			pgx.Identifier.Sanitize([]string{graphLookupName})),
	}

	for _, sql := range statements {
		if _, err := tx.Exec(ctx, sql); err != nil {
			return errors.Wrap(err, "error while swapping in the graph table")
		}
	}

	return nil
}

func (repo GraphBuilderRepositoryPostgresql) DropGraphBuildTable(ctx context.Context, exec Executor) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := fmt.Sprintf("drop table if exists %s", pgIdentifierWithSchema(exec, graphBuildTable))
	if _, err := exec.Exec(ctx, sql); err != nil {
		return errors.Wrap(err, "error while dropping the graph build table")
	}

	return nil
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
		// Casting a timestamptz renders it in the session's TimeZone, so the same instant would
		// be written differently depending on where the worker happened to run.
		return fmt.Sprintf("(%s at time zone 'utc')::text", column)
	default:
		return column + "::text"
	}
}

// pgStringLiteral quotes a value for inclusion in a statement that cannot take a parameter. Only
// the record type and field names go through it, both of which the data model has already
// constrained to `^[a-z][a-z0-9_]{0,62}$`; the quoting is what keeps that from being the only
// thing standing between the data model and injected SQL.
func pgStringLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
