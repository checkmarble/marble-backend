package repositories

import (
	"context"
	"encoding/json"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
)

type GraphRepository interface {
	GetNodeRows(ctx context.Context, exec Executor, recordType, recordId string) ([]models.GraphRow, error)
	FindMatchingRows(ctx context.Context, exec Executor, recordType, fieldName, fieldValue string, limit int) ([]models.GraphRow, error)
	EstimateMatchingRows(ctx context.Context, exec Executor, recordType, fieldName, fieldValue string) (int, error)
}

type GraphRepositoryPostgresql struct{}

// GetNodeRows returns every `_graph` row (field_name/field_value pair) for a node.
func (repo GraphRepositoryPostgresql) GetNodeRows(
	ctx context.Context, exec Executor, recordType, recordId string,
) ([]models.GraphRow, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	q := graphSelect(exec).Where(squirrel.Eq{
		"record_type": recordType,
		"record_id":   recordId,
	})
	return queryGraphRows(ctx, exec, q)
}

// FindMatchingRows returns up to `limit` `_graph` rows of the given type/field
// whose value equals fieldValue — i.e. the other endpoint of a value-match edge.
// The caller uses the limit both to bound a single lookup and to detect a
// hyperconnected relationship (by fetching threshold+1 and checking for overflow).
func (repo GraphRepositoryPostgresql) FindMatchingRows(
	ctx context.Context, exec Executor, recordType, fieldName, fieldValue string, limit int,
) ([]models.GraphRow, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	q := graphSelect(exec).
		Where(graphMatchPredicate(recordType, fieldName, fieldValue)).
		Limit(uint64(limit))
	return queryGraphRows(ctx, exec, q)
}

// EstimateMatchingRows returns the Postgres planner's estimated number of
// `_graph` rows matching (recordType, fieldName, fieldValue). It is intentionally
// approximate: EXPLAIN reads the table's ANALYZE statistics instead of scanning,
// so it is near-instant even for a value shared by millions of rows, and its
// coarseness at high volume is acceptable (only used to size a hyperconnected
// node's pruned fan-out, where order of magnitude is what matters).
func (repo GraphRepositoryPostgresql) EstimateMatchingRows(
	ctx context.Context, exec Executor, recordType, fieldName, fieldValue string,
) (int, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return 0, err
	}

	q := NewQueryBuilder().
		Select("1").
		From(pgIdentifierWithSchema(exec, "_graph")).
		Where(graphMatchPredicate(recordType, fieldName, fieldValue))
	sql, args, err := q.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "error while building _graph estimate query")
	}

	// EXPLAIN plans with the supplied parameter values (a custom plan), so the
	// estimate is specific to this field_value rather than a generic average.
	var raw []byte
	if err := exec.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+sql, args...).Scan(&raw); err != nil {
		return 0, errors.Wrap(err, "error while estimating _graph matches")
	}

	var plans []struct {
		Plan struct {
			PlanRows float64 `json:"Plan Rows"`
		} `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &plans); err != nil {
		return 0, errors.Wrap(err, "error while parsing _graph estimate plan")
	}
	if len(plans) == 0 {
		return 0, nil
	}
	return int(plans[0].Plan.PlanRows), nil
}

func graphMatchPredicate(recordType, fieldName, fieldValue string) squirrel.Eq {
	return squirrel.Eq{
		"record_type": recordType,
		"field_name":  fieldName,
		"field_value": fieldValue,
	}
}

func graphSelect(exec Executor) squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select("record_type", "record_id", "field_name", "field_value").
		From(pgIdentifierWithSchema(exec, "_graph"))
}

func queryGraphRows(ctx context.Context, exec Executor, q squirrel.SelectBuilder) ([]models.GraphRow, error) {
	sql, args, err := q.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error while building _graph query")
	}

	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error while querying _graph")
	}
	defer rows.Close()

	output := make([]models.GraphRow, 0)
	for rows.Next() {
		var r models.GraphRow
		if err := rows.Scan(&r.RecordType, &r.RecordId, &r.FieldName, &r.FieldValue); err != nil {
			return nil, errors.Wrap(err, "error while scanning _graph row")
		}
		output = append(output, r)
	}
	return output, rows.Err()
}
