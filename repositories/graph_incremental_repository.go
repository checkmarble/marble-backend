package repositories

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
)

// The adjacency table is rebuilt wholesale on a schedule, which leaves a record ingested just after
// a build invisible to graph walks until the next one. These two statements keep it current for the
// records ingestion touches, so the periodic rebuild becomes a backstop rather than the only source
// of freshness.
type GraphIncrementalRepository interface {
	UpsertGraphRows(ctx context.Context, exec Executor, recordType string, fields []models.Field, objectIds []string) (int64, error)
	RetractGraphRows(ctx context.Context, exec Executor, recordType string, fields []models.Field, objectIds []string) (int64, error)
}

func (repo MarbleDbRepository) UpsertGraphRows(ctx context.Context, exec Executor, recordType string, fields []models.Field, objectIds []string) (int64, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return 0, err
	}
	if len(fields) == 0 || len(objectIds) == 0 {
		return 0, nil
	}

	// The inequality guard keeps re-ingesting an unchanged record from costing an index write, since
	// idx_graph_lookup covers field_value. Plain `<>` is enough: field_value is declared not null and
	// the filter below drops null projections, so there is no null for it to mishandle.
	sql := fmt.Sprintf(`
		insert into %s as g (record_type, record_id, field_name, field_value, updated_at)
		select %s, t.object_id, v.field_name, v.field_value, clock_timestamp()
		from %s t
		cross join lateral (values %s) as v(field_name, field_value)
		where t.valid_until = 'infinity'
			and t.object_id = any($1)
			and v.field_value is not null
			and v.field_value <> ''
		on conflict (record_type, record_id, field_name) do update
			set field_value = excluded.field_value,
				updated_at = excluded.updated_at
			where g.field_value <> excluded.field_value`,
		pgIdentifierWithSchema(exec, graphTable),
		pgClientDataIdentifierString(recordType),
		pgIdentifierWithSchema(exec, recordType),
		graphFieldUnpivot(fields))

	tag, err := exec.Exec(ctx, sql, objectIds)
	if err != nil {
		return 0, errors.Wrapf(err, "error while upserting graph rows for %q", recordType)
	}

	return tag.RowsAffected(), nil
}

func (repo MarbleDbRepository) RetractGraphRows(ctx context.Context, exec Executor, recordType string, fields []models.Field, objectIds []string) (int64, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return 0, err
	}
	// An empty field list would make the NOT EXISTS vacuously true and delete every row of these
	// records. GraphIndexedFields never returns one — object_id is always included — but the
	// consequence of being wrong here is silent data loss, so it is checked rather than assumed.
	if len(fields) == 0 || len(objectIds) == 0 {
		return 0, nil
	}

	sql := fmt.Sprintf(`
		delete from %s g
		where g.record_type = %s
			and g.record_id = any($1)
			and not exists (
				select 1
				from %s t
				cross join lateral (values %s) as v(field_name, field_value)
				where t.valid_until = 'infinity'
					and t.object_id = g.record_id
					and v.field_name = g.field_name
					and v.field_value is not null
					and v.field_value <> ''
			)`,
		pgIdentifierWithSchema(exec, graphTable),
		pgClientDataIdentifierString(recordType),
		pgIdentifierWithSchema(exec, recordType),
		graphFieldUnpivot(fields))

	tag, err := exec.Exec(ctx, sql, objectIds)
	if err != nil {
		return 0, errors.Wrapf(err, "error while retracting graph rows for %q", recordType)
	}

	return tag.RowsAffected(), nil
}
