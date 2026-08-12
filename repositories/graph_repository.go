package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

// graphTable is the client-schema adjacency table the graph walk reads. It holds one row per
// (record, participating field, value) and is maintained out of band by a dedicated worker,
// so nothing here writes to it. The walk assumes it only represents live records — there is
// no valid_until column to filter on — and that it is indexed on
// (record_type, field_name, field_value) and (record_type, record_id), which are the two
// query shapes below.
const graphTable = "_graph"

// graphBatchSize bounds how many ids or values go into a single array parameter, so one
// query's array stays a size Postgres plans sensibly.
const graphBatchSize = 1000

type GraphRepository interface {
	// FetchFields returns the `_graph` rows of recordType for the given record ids,
	// restricted to fieldNames. One call hydrates a whole frontier of one record type.
	FetchFields(ctx context.Context, exec Executor, recordType string, recordIds, fieldNames []string) ([]models.GraphRow, error)

	// FindByValues returns the records of recordType whose fieldName is one of values, at
	// most perValueLimit of them per requested value. Pass the walk's cap plus one to tell
	// "exactly at the cap" from "over it".
	FindByValues(ctx context.Context, exec Executor, recordType, fieldName string, values []string, perValueLimit int) ([]models.GraphMatch, error)

	// EstimateValueCount returns the planner's estimate of how many records carry a value.
	// It is only called for a relationship already known to be over its cap, to put a
	// number on it: EXPLAIN reads the table's ANALYZE statistics instead of scanning, so it
	// stays near-instant even for a value shared by millions of records, and its coarseness
	// at that volume is acceptable when order of magnitude is all that is reported. Being
	// sampled, it also lands under the truth often enough that callers should treat it as an
	// approximation and never as an upper bound.
	EstimateValueCount(ctx context.Context, exec Executor, recordType, fieldName, value string) (int, error)

	GetNodeBatchMetadata(
		ctx context.Context,
		exec Executor,
		orgId uuid.UUID,
		records []models.ScoringRecordRef,
	) ([]models.GraphResultNodeMetadata, error)
}

func (repo MarbleDbRepository) FetchFields(
	ctx context.Context,
	exec Executor,
	recordType string,
	recordIds, fieldNames []string,
) ([]models.GraphRow, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}
	if len(recordIds) == 0 || len(fieldNames) == 0 {
		return nil, nil
	}

	output := make([]models.GraphRow, 0, len(recordIds)*len(fieldNames))

	for ids := range slices.Chunk(recordIds, graphBatchSize) {
		q := NewQueryBuilder().
			Select("record_id", "field_name", "field_value").
			From(pgIdentifierWithSchema(exec, graphTable)).
			Where(squirrel.Eq{"record_type": recordType}).
			Where("record_id = ANY(?)", ids).
			Where("field_name = ANY(?)", fieldNames)

		err := ForEachRow(ctx, exec, q, func(row pgx.CollectableRow) error {
			var r models.GraphRow

			if err := row.Scan(&r.RecordId, &r.FieldName, &r.FieldValue); err != nil {
				return errors.Wrap(err, "error while scanning _graph row")
			}

			output = append(output, r)

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return output, nil
}

func (repo MarbleDbRepository) FindByValues(
	ctx context.Context,
	exec Executor,
	recordType, fieldName string,
	values []string,
	perValueLimit int,
) ([]models.GraphMatch, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}
	if len(values) == 0 || perValueLimit <= 0 {
		return nil, nil
	}

	// The LIMIT sits inside the LATERAL, so it caps rows *per requested value*: a value
	// carried by millions of records stops at perValueLimit instead of materialising, while
	// every value still gets looked up in a single round trip. A grouped count would have
	// to read all of those rows before it could tell us to skip them.
	sql := fmt.Sprintf(`
		select v.val, s.record_id
		from unnest($1::text[]) as v(val)
		cross join lateral (
			select record_id
			from %s
			where record_type = $2 and field_name = $3 and field_value = v.val
			limit $4
		) s`, pgIdentifierWithSchema(exec, graphTable))

	output := make([]models.GraphMatch, 0, len(values))

	for batch := range slices.Chunk(values, graphBatchSize) {
		if err := repo.collectMatches(ctx, exec, sql, batch, recordType, fieldName, perValueLimit, &output); err != nil {
			return nil, err
		}
	}

	return output, nil
}

func (repo MarbleDbRepository) collectMatches(
	ctx context.Context,
	exec Executor,
	sql string,
	values []string,
	recordType, fieldName string,
	perValueLimit int,
	output *[]models.GraphMatch,
) error {
	rows, err := exec.Query(ctx, sql, values, recordType, fieldName, perValueLimit)
	if err != nil {
		return errors.Wrap(err, "error while querying _graph by field values")
	}
	defer rows.Close()

	for rows.Next() {
		var m models.GraphMatch

		if err := rows.Scan(&m.Value, &m.RecordId); err != nil {
			return errors.Wrap(err, "error while scanning _graph match")
		}

		*output = append(*output, m)
	}

	return errors.Wrap(rows.Err(), "error while iterating over _graph matches")
}

func (repo MarbleDbRepository) EstimateValueCount(
	ctx context.Context,
	exec Executor,
	recordType, fieldName, value string,
) (int, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return 0, err
	}

	q := NewQueryBuilder().
		Select("1").
		From(pgIdentifierWithSchema(exec, graphTable)).
		Where(squirrel.Eq{
			"record_type": recordType,
			"field_name":  fieldName,
			"field_value": value,
		})
	sql, args, err := q.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "error while building _graph estimate query")
	}

	// EXPLAIN plans with the supplied parameter values, so the estimate is specific to this
	// value rather than a generic average over the column.
	var raw []byte
	if err := exec.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+sql, args...).Scan(&raw); err != nil {
		return 0, errors.Wrap(err, "error while estimating _graph matches")
	}

	var plans []struct {
		Plan struct {
			PlanRows float64 `json:"Plan Rows"` //nolint:tagliatelle
		} `json:"Plan"` //nolint:tagliatelle
	}

	if err := json.Unmarshal(raw, &plans); err != nil {
		return 0, errors.Wrap(err, "error while parsing _graph estimate plan")
	}
	if len(plans) == 0 {
		return 0, nil
	}

	return int(plans[0].Plan.PlanRows), nil
}

func (repo MarbleDbRepository) GetNodeBatchMetadata(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	records []models.ScoringRecordRef,
) ([]models.GraphResultNodeMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	types := make([]string, len(records))
	ids := make([]string, len(records))

	for idx, record := range records {
		types[idx] = record.RecordType
		ids[idx] = record.RecordId
	}

	sql := fmt.Sprintf(
		`
			with
		  inputs as (
		    select type, id, ord
		    from unnest($2::text[], $3::text[])
		      with ordinality as input(type, id, ord)
		  ),
		  tags as (
		    select tt.object_type, tt.object_id, array_agg(tt.payload->>'tag_id') AS ids
		    from %s tt
		    inner join inputs i on
					tt.annotation_type = 'tag' and
		      tt.org_id = $1 and
		      tt.object_type = i.type and
		      tt.object_id = i.id
		    group by tt.object_type, tt.object_id
		  )
			select
		    i.ord,
		    ts.risk_level,
		    tt.ids as tags
			from inputs i
			left join %s ts on
			  ts.org_id = $1 and
			  ts.record_type = i.type and
			  ts.record_id = i.id and
				ts.deleted_at is null
			left join tags tt on
			  tt.object_type = i.type and
			  tt.object_id = i.id;
		`,
		dbmodels.TABLE_ENTITY_ANNOTATIONS,
		dbmodels.TABLE_SCORING_SCORES,
	)

	rows, err := exec.Query(ctx, sql, orgId, types, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dbMetadata, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbmodels.DbGraphOrderedMetadata])
	if err != nil {
		return nil, err
	}

	return pure_utils.MapErr(dbMetadata, dbmodels.AdaptGraphOrderedMetadata)
}
