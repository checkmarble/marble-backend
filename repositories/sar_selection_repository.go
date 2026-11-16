package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

// ListSarSelection returns the selection of a declaration, oldest first (ties
// broken by table then object id, so the order is deterministic).
func (repo *MarbleDbRepository) ListSarSelection(
	ctx context.Context, exec Executor, declarationId uuid.UUID,
) ([]models.SarSelectedObject, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SarSelectionColumns...).
		From(dbmodels.TABLE_SAR_DECLARATION_SELECTIONS).
		Where(squirrel.Eq{"declaration_id": declarationId}).
		OrderBy("created_at ASC", "table_name ASC", "object_id ASC")
	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptSarSelectedObject)
}

// AddSarSelection inserts the objects in one statement. A (declaration, table,
// object) already selected returns models.ConflictError and nothing is written.
func (repo *MarbleDbRepository) AddSarSelection(
	ctx context.Context, exec Executor, objs []models.SarSelectedObject,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	if len(objs) == 0 {
		return nil
	}
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SAR_DECLARATION_SELECTIONS).
		Columns("id", "declaration_id", "table_name", "object_id", "snapshot", "origin", "reached_from", "created_at")
	now := time.Now()
	for _, o := range objs {
		snapshot, err := json.Marshal(o.Snapshot)
		if err != nil {
			return errors.Wrap(err, "cannot encode sar selection snapshot")
		}
		origin, err := json.Marshal(o.Origin)
		if err != nil {
			return errors.Wrap(err, "cannot encode sar selection origin")
		}
		var reached any // SQL NULL for a direct candidate
		if o.ReachedFrom != nil {
			raw, err := json.Marshal(o.ReachedFrom)
			if err != nil {
				return errors.Wrap(err, "cannot encode sar selection reached_from")
			}
			reached = json.RawMessage(raw)
		}
		createdAt := o.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		query = query.Values(o.Id, o.DeclarationId, o.TableName, o.ObjectId,
			json.RawMessage(snapshot), json.RawMessage(origin), reached, createdAt)
	}
	err := ExecBuilder(ctx, exec, query)
	if IsUniqueViolationError(err) {
		return errors.Wrap(models.ConflictError, "objet déjà sélectionné pour cette déclaration")
	}
	return err
}

// RemoveSarSelection deletes the selected objects matching refs (absent refs
// are ignored).
func (repo *MarbleDbRepository) RemoveSarSelection(
	ctx context.Context, exec Executor, declarationId uuid.UUID, refs []models.CaseEntityRef,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}
	match := squirrel.Or{}
	for _, r := range refs {
		match = append(match, squirrel.Eq{"table_name": r.TableName, "object_id": r.ObjectId})
	}
	query := NewQueryBuilder().
		Delete(dbmodels.TABLE_SAR_DECLARATION_SELECTIONS).
		Where(squirrel.Eq{"declaration_id": declarationId}).
		Where(match)
	return ExecBuilder(ctx, exec, query)
}
