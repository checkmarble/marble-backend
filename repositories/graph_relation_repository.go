package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type GraphRelationRepository interface {
	ListGraphRelations(ctx context.Context, exec Executor, orgId uuid.UUID) ([]models.GraphRelation, error)
	GetGraphRelationGroupLabel(ctx context.Context, exec Executor, orgId, groupId uuid.UUID) (string, error)
	GetGraphRelation(ctx context.Context, exec Executor, id uuid.UUID) (models.GraphRelation, error)
	CreateGraphRelation(ctx context.Context, exec Executor, relation models.CreateGraphRelation) (models.GraphRelation, error)
	DeleteGraphRelation(ctx context.Context, exec Executor, id uuid.UUID) error
}

func (repo *MarbleDbRepository) ListGraphRelations(
	ctx context.Context, exec Executor, orgId uuid.UUID,
) ([]models.GraphRelation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectGraphRelationColumn...).
		From(dbmodels.TABLE_GRAPH_RELATIONS).
		Where(squirrel.Eq{"org_id": orgId}).
		// The walk's output order follows the order relations are registered in, so this is what
		// keeps two walks over identical data returning identical graphs.
		OrderBy("created_at", "id")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptGraphRelation)
}

func (repo *MarbleDbRepository) GetGraphRelationGroupLabel(ctx context.Context, exec Executor, orgId, groupId uuid.UUID) (string, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return "", err
	}

	sql := NewQueryBuilder().
		Select("label").
		From(dbmodels.TABLE_GRAPH_RELATIONS).
		Where(squirrel.Eq{"org_id": orgId, "group_id": groupId}).
		Limit(1)

	query, args, err := sql.ToSql()
	if err != nil {
		return "", err
	}

	row := exec.QueryRow(ctx, query, args...)

	var label string

	if err := row.Scan(&label); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.Wrap(models.NotFoundError, "provided group does not exist")
		}

		return "", err
	}

	return label, nil
}

func (repo *MarbleDbRepository) GetGraphRelation(
	ctx context.Context, exec Executor, id uuid.UUID,
) (models.GraphRelation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.GraphRelation{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectGraphRelationColumn...).
		From(dbmodels.TABLE_GRAPH_RELATIONS).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptGraphRelation)
}

func (repo *MarbleDbRepository) CreateGraphRelation(ctx context.Context, exec Executor, relation models.CreateGraphRelation) (models.GraphRelation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.GraphRelation{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_GRAPH_RELATIONS).
		Columns("id", "org_id", "group_id", "label", "left_type", "left_field", "right_type", "right_field").
		Values(
			pure_utils.NewId(),
			relation.OrgId,
			relation.GroupId,
			relation.Label,
			relation.LeftType,
			relation.LeftField,
			relation.RightType,
			relation.RightField,
		).
		Suffix(fmt.Sprintf("returning %s", strings.Join(dbmodels.SelectGraphRelationColumn, ",")))

	return SqlToModel(ctx, exec, query, dbmodels.AdaptGraphRelation)
}

func (repo *MarbleDbRepository) DeleteGraphRelation(ctx context.Context, exec Executor, id uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	return ExecBuilder(ctx, exec,
		NewQueryBuilder().Delete(dbmodels.TABLE_GRAPH_RELATIONS).Where(squirrel.Eq{"id": id}))
}
