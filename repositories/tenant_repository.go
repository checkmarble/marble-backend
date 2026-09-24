package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TenantRepository interface {
	ListActive(ctx context.Context, exec Executor) ([]models.Tenant, error)
	LockAndValidateMerge(ctx context.Context, exec Executor, targetId uuid.UUID, sourceIds []uuid.UUID) error
	LockAndValidateUpdate(ctx context.Context, exec Executor, tenantId uuid.UUID) error
	SoftDelete(ctx context.Context, exec Executor, tenantIds []uuid.UUID) error
	Rename(ctx context.Context, exec Executor, tenantId uuid.UUID, name string) error
}

func (repo *MarbleDbRepository) ListActive(ctx context.Context, exec Executor) ([]models.Tenant, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	return SqlToListOfRow(ctx, exec, NewQueryBuilder().
		Select("id", "name").
		From("tenants").
		Where(squirrel.Eq{"deleted_at": nil}).
		OrderBy("name", "id"), func(row pgx.CollectableRow) (models.Tenant, error) {
		var tenant models.Tenant
		if err := row.Scan(&tenant.Id, &tenant.Name); err != nil {
			return models.Tenant{}, fmt.Errorf("scanning tenant: %w", err)
		}
		return tenant, nil
	})
}

func (repo *MarbleDbRepository) LockAndValidateMerge(ctx context.Context, exec Executor, targetId uuid.UUID, sourceIds []uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	tenantIds := append([]uuid.UUID{targetId}, sourceIds...)
	query := NewQueryBuilder().
		Select("id", "deleted_at IS NOT NULL").
		From("tenants").
		Where(squirrel.Eq{"id": tenantIds}).
		OrderBy("id").
		Suffix("FOR UPDATE")

	found := make(map[uuid.UUID]bool, len(tenantIds))
	deleted := make(map[uuid.UUID]bool, len(tenantIds))
	err := ForEachRow(ctx, exec, query, func(row pgx.CollectableRow) error {
		var id uuid.UUID
		var isDeleted bool
		if err := row.Scan(&id, &isDeleted); err != nil {
			return fmt.Errorf("scanning tenant: %w", err)
		}
		found[id] = true
		deleted[id] = isDeleted
		return nil
	})
	if err != nil {
		return err
	}
	if !found[targetId] || deleted[targetId] {
		return models.NotFoundError
	}
	for _, sourceId := range sourceIds {
		if !found[sourceId] {
			return models.NotFoundError
		}
	}
	return nil
}

func (repo *MarbleDbRepository) LockAndValidateUpdate(ctx context.Context, exec Executor, tenantId uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	_, err := SqlToRow(ctx, exec, NewQueryBuilder().
		Select("id").
		From("tenants").
		Where(squirrel.And{
			squirrel.Eq{"id": tenantId},
			squirrel.Eq{"deleted_at": nil},
		}).
		Suffix("FOR UPDATE"), func(row pgx.CollectableRow) (uuid.UUID, error) {
		var id uuid.UUID
		return id, row.Scan(&id)
	})
	return err
}

func (repo *MarbleDbRepository) SoftDelete(ctx context.Context, exec Executor, tenantIds []uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	return ExecBuilder(ctx, exec, NewQueryBuilder().
		Update("tenants").
		Set("deleted_at", squirrel.Expr("now()")).
		Where(squirrel.And{
			squirrel.Eq{"id": tenantIds},
			squirrel.Eq{"deleted_at": nil},
		}))
}

func (repo *MarbleDbRepository) Rename(ctx context.Context, exec Executor, tenantId uuid.UUID, name string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	return ExecBuilder(ctx, exec, NewQueryBuilder().
		Update("tenants").
		Set("name", name).
		Where(squirrel.Eq{"id": tenantId}))
}
