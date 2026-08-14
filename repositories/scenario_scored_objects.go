package repositories

import (
	"context"
	"sort"

	"github.com/jackc/pgx/v5"
)

const TABLE_SCENARIO_SCORED_OBJECTS = "scenario_scored_objects"

// FilterAlreadyScoredObjects returns which of the given objects have already been claimed.
// A pre-filter only: it exists to skip the expensive part (ingested object read + full
// scenario evaluation), never to decide correctness -- ClaimScoredObjects does that.
//
// Safe to run outside any transaction, under Read Committed, because the table is
// append-only: a committed claim is permanent. So this can only produce false negatives
// (missing a claim still in flight, costing one wasted evaluation that the authoritative
// claim then rejects), never false positives (reporting scored for an object that ends up
// unscored).
func (repo *MarbleDbRepository) FilterAlreadyScoredObjects(
	ctx context.Context,
	exec Executor,
	scenarioId string,
	objectIds []string,
) (map[string]struct{}, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	if len(objectIds) == 0 {
		return nil, nil
	}

	query := NewQueryBuilder().
		Select("object_id").
		From(TABLE_SCENARIO_SCORED_OBJECTS).
		Where("scenario_id = ?", scenarioId).
		Where("object_id = ANY(?)", objectIds)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	alreadyScored := make(map[string]struct{})
	var objectId string
	for rows.Next() {
		if err := rows.Scan(&objectId); err != nil {
			return nil, err
		}
		alreadyScored[objectId] = struct{}{}
	}
	return alreadyScored, rows.Err()
}

// ClaimScoredObjects atomically reserves the right to create a decision for each of the given
// objects, and returns the subset actually reserved. Only those may be turned into decisions.
//
// This is a mutual-exclusion primitive, not a "tolerate duplicates" upsert. On a unique
// conflict with an in-progress transaction, PostgreSQL waits for that transaction to
// commit or abort before deciding -- and that wait happens before the DO NOTHING shortcut.
// RETURNING on DO NOTHING yields only rows actually inserted (unlike DO UPDATE), so the
// returned set is the arbitration result: committed competitor -> excluded, aborted
// competitor -> included.
//
// MUST be called on a Transaction, never on the pool executor, and in the same transaction
// as the decision writes and the manifest advance. The claim inherits that transaction's
// atomicity: a rollback un-claims, so a retry re-evaluates and re-claims. Claiming outside
// the transaction would publish reservations a rollback cannot undo, permanently excluding
// objects that never got a decision.
//
// Ids are sorted here so that two concurrent runs acquire the conflicting keys in the same
// order and cannot deadlock. The transaction factory only retries deadlocks once
// (db_executor_getter.go), and each retry would replay the whole batch write.
//
// The conflict target is explicit so that adding another unique index to this table later
// cannot silently swallow an unrelated violation.
//
// Duplicate ids within one call are safe: DO NOTHING skips the second occurrence rather
// than raising (unlike DO UPDATE). That happens if the client table holds more than one
// valid row per object_id, in which case this also collapses intra-run duplicates.
func (repo *MarbleDbRepository) ClaimScoredObjects(
	ctx context.Context,
	tx Transaction,
	scenarioId string,
	objectIds []string,
) ([]string, error) {
	if len(objectIds) == 0 {
		return nil, nil
	}

	sorted := append([]string(nil), objectIds...)
	sort.Strings(sorted)

	query := NewQueryBuilder().
		Insert(TABLE_SCENARIO_SCORED_OBJECTS).
		Columns("scenario_id", "object_id")
	for _, objectId := range sorted {
		query = query.Values(scenarioId, objectId)
	}
	query = query.Suffix(
		"ON CONFLICT (scenario_id, object_id) DO NOTHING RETURNING object_id")

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	claimed := make([]string, 0, len(sorted))
	for rows.Next() {
		objectId, err := pgx.RowTo[string](rows)
		if err != nil {
			return nil, err
		}
		claimed = append(claimed, objectId)
	}
	return claimed, rows.Err()
}
