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
	req models.AnnotationByIdRequest,
) ([]models.EntityAnnotation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	filters := squirrel.Eq{
		"id":         req.AnnotationId,
		"org_id":     req.OrgId,
		"deleted_at": nil,
	}

	sql := NewQueryBuilder().
		Select(dbmodels.EntityAnnotationColumns...).
		From(dbmodels.TABLE_ENTITY_ANNOTATIONS).
		Where(filters)

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptEntityAnnotation)
}

func (repo MarbleDbRepository) GetEntityAnnotations(
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
		From(dbmodels.TABLE_ENTITY_ANNOTATIONS).
		Where(filters)

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptEntityAnnotation)
}

func (repo MarbleDbRepository) GetEntityAnnotationsForObjects(
	ctx context.Context,
	exec Executor,
	req models.EntityAnnotationRequestForObjects,
) (map[string][]models.EntityAnnotation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	filters := squirrel.Eq{
		"org_id":      req.OrgId,
		"object_type": req.ObjectType,
		"deleted_at":  nil,
	}

	if req.AnnotationType != nil {
		filters["annotation_type"] = req.AnnotationType.String()
	}

	sql := NewQueryBuilder().
		Select(dbmodels.EntityAnnotationColumns...).
		From(dbmodels.TABLE_ENTITY_ANNOTATIONS).
		Where(filters).
		Where("object_id = ANY(?)", req.ObjectIds)

	annotations, err := SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptEntityAnnotation)
	if err != nil {
		return nil, err
	}

	annotationsByObject := make(map[string][]models.EntityAnnotation)

	for _, ann := range annotations {
		if _, ok := annotationsByObject[ann.ObjectId]; !ok {
			annotationsByObject[ann.ObjectId] = make([]models.EntityAnnotation, 0)
		}

		annotationsByObject[ann.ObjectId] = append(annotationsByObject[ann.ObjectId], ann)
	}

	return annotationsByObject, nil
}

func (repo MarbleDbRepository) GetEntityAnnotationsForCase(
	ctx context.Context,
	exec Executor,
	req models.CaseEntityAnnotationRequest,
) ([]models.EntityAnnotation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	filters := squirrel.Eq{
		"org_id":     req.OrgId,
		"case_id":    req.CaseId,
		"deleted_at": nil,
	}

	if req.AnnotationType != nil {
		filters["annotation_type"] = req.AnnotationType.String()
	}

	sql := NewQueryBuilder().
		Select(dbmodels.EntityAnnotationColumns...).
		From(dbmodels.TABLE_ENTITY_ANNOTATIONS).
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
		Insert(dbmodels.TABLE_ENTITY_ANNOTATIONS).
		Columns("org_id", "object_type", "object_id", "case_id", "annotation_type", "payload", "annotated_by").
		Values(
			req.OrgId,
			req.ObjectType,
			req.ObjectId,
			req.CaseId,
			req.AnnotationType,
			req.Payload,
			req.AnnotatedBy,
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
		Select("1").
		From(dbmodels.TABLE_ENTITY_ANNOTATIONS).
		Where(filters).
		Limit(1).
		Prefix("select exists (").Suffix(")")

	sql, args, err := query.ToSql()
	if err != nil {
		return false, err
	}

	var hasTag bool

	if err := exec.QueryRow(ctx, sql, args...).Scan(&hasTag); err != nil {
		return false, err
	}

	return hasTag, nil
}

func (repo *MarbleDbRepository) DeleteEntityAnnotation(ctx context.Context, exec Executor,
	req models.AnnotationByIdRequest,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	filters := squirrel.Eq{
		"id":         req.AnnotationId,
		"org_id":     req.OrgId,
		"deleted_at": nil,
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_ENTITY_ANNOTATIONS).
		Set("deleted_at", "now()").
		Where(filters)

	return ExecBuilder(ctx, exec, sql)
}
