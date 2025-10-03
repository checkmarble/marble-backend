package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repo *MarbleDbRepository) GetMassCasesByIds(ctx context.Context, exec Executor, caseIds []uuid.UUID) ([]models.Case, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseColumn...).
		From(dbmodels.TABLE_CASES).
		Where(squirrel.Eq{"id": caseIds})

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptCase)
}

func (repo *MarbleDbRepository) CaseMassChangeStatus(ctx context.Context, tx Transaction, caseIds []uuid.UUID, status models.CaseStatus) ([]uuid.UUID, error) {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_CASES).
		Set("status", status).
		Set("boost", nil).
		Where(squirrel.And{
			squirrel.Eq{"id": caseIds},
			squirrel.NotEq{"status": status},
		}).
		Suffix("returning id")

	return caseMassUpdateExecAndReturnedChanged(ctx, tx, query)
}

func (repo *MarbleDbRepository) CaseMassAssign(ctx context.Context, tx Transaction, caseIds []uuid.UUID, assigneeId uuid.UUID) ([]uuid.UUID, error) {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_CASES).
		Set("assigned_to", assigneeId).
		Set("boost", nil).
		Where(squirrel.And{
			squirrel.Eq{"id": caseIds},
			squirrel.Or{
				squirrel.Eq{"assigned_to": nil},
				squirrel.NotEq{"assigned_to": assigneeId},
			},
		}).
		Suffix("returning id")

	return caseMassUpdateExecAndReturnedChanged(ctx, tx, query)
}

func (repo *MarbleDbRepository) CaseMassMoveToInbox(ctx context.Context, tx Transaction, caseIds []uuid.UUID, inboxId uuid.UUID) ([]uuid.UUID, error) {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_CASES).
		Set("inbox_id", inboxId).
		Set("boost", nil).
		Where(squirrel.And{
			squirrel.Eq{"id": caseIds},
			squirrel.Or{
				squirrel.Eq{"inbox_id": nil},
				squirrel.NotEq{"inbox_id": inboxId},
			},
		}).
		Suffix("returning id")

	return caseMassUpdateExecAndReturnedChanged(ctx, tx, query)
}

func caseMassUpdateExecAndReturnedChanged(ctx context.Context, tx Transaction, query squirrel.UpdateBuilder) ([]uuid.UUID, error) {
	sql, args, err := query.ToSql()

	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, sql, args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var tmp uuid.UUID

	ids, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (uuid.UUID, error) {
		if err := row.Scan(&tmp); err != nil {
			return uuid.Nil, err
		}

		return tmp, nil
	})

	if err != nil {
		return nil, err
	}

	return ids, nil
}
