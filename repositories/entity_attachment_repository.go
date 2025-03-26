package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) GetEntityAnnotationById(
	ctx context.Context,
	exec Executor,
	req models.EntityAnnotationRequest,
	id string,
) ([]models.EntityAnnotation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	filters := squirrel.Eq{
		"id":          id,
		"org_id":      req.OrgId,
		"object_type": req.ObjectType,
		"object_id":   req.ObjectId,
		"deleted_at":  nil,
	}

	if req.AnnotationType != nil {
		filters["annotation_type"] = req.AnnotationType.String()
	}

	sql := NewQueryBuilder().
		Select(dbmodels.EntityAnnotationColumns...).
		From(dbmodels.TABLE_ENTITY_ATTACHMENTS).
		Where(filters)

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptEntityAnnotation)
}

func (repo *MarbleDbRepository) GetEntityAnnotations(
	ctx context.Context,
	exec Executor,
	req models.EntityAnnotationRequest,
) ([]models.EntityAnnotation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	filters := squirrel.Eq{
		"org_id":      req.OrgId,
		"object_type": req.ObjectType,
		"object_id":   req.ObjectId,
		"deleted_at":  nil,
	}

	if req.AnnotationType != nil {
		filters["annotation_type"] = req.AnnotationType.String()
	}

	sql := NewQueryBuilder().
		Select(dbmodels.EntityAnnotationColumns...).
		From(dbmodels.TABLE_ENTITY_ATTACHMENTS).
		Where(filters)

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptEntityAnnotation)
}

func (repo *MarbleDbRepository) CreateEntityAnnotation(
	ctx context.Context,
	exec Executor,
	req models.CreateEntityAnnotationRequest,
) (models.EntityAnnotation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.EntityAnnotation{}, err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_ENTITY_ATTACHMENTS).
		Columns("org_id", "object_type", "object_id", "annotation_type", "payload", "attached_by").
		Values(
			req.OrgId,
			req.ObjectType,
			req.ObjectId,
			req.AnnotationType,
			req.Payload,
			req.AttachedBy,
		).
		Suffix("returning *")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptEntityAnnotation)
}

func (repo *MarbleDbRepository) IsObjectTagSet(ctx context.Context, exec Executor,
	req models.CreateEntityAnnotationRequest, tagId string,
) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}

	filters := squirrel.Eq{
		"org_id":          req.OrgId,
		"object_type":     req.ObjectType,
		"annotation_type": models.EntityAnnotationTag,
		"object_id":       req.ObjectId,
		"deleted_at":      nil,
		"payload->>'tag'": tagId,
	}

	query := NewQueryBuilder().
		Select("count(1)").
		From(dbmodels.TABLE_ENTITY_ATTACHMENTS).
		Where(filters)

	sql, args, err := query.ToSql()
	if err != nil {
		return false, err
	}

	row := exec.QueryRow(ctx, sql, args...)

	var tagCount int

	if err := row.Scan(&tagCount); err != nil {
		return false, err
	}

	return tagCount > 0, nil
}
