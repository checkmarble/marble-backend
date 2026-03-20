package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
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
		"id":     req.AnnotationId,
		"org_id": req.OrgId,
	}

	if !req.IncludeDeleted {
		filters["deleted_at"] = nil
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
		Columns("id", "org_id", "object_type", "object_id", "case_id", "annotation_type", "payload", "annotated_by").
		Values(
			uuid.Must(uuid.NewV7()).String(),
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

func (repo *MarbleDbRepository) FindExistingObjectTagAnnotation(ctx context.Context, exec Executor,
	req models.CreateEntityAnnotationRequest, tagId string,
) (models.EntityAnnotation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.EntityAnnotation{}, err
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
		Select(dbmodels.EntityAnnotationColumns...).
		From(dbmodels.TABLE_ENTITY_ANNOTATIONS).
		Where(filters).
		Limit(1)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptEntityAnnotation)
}

func (repo *MarbleDbRepository) FindExistingObjectRiskTagAnnotation(ctx context.Context, exec Executor,
	req models.CreateEntityAnnotationRequest, tag string,
) (models.EntityAnnotation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.EntityAnnotation{}, err
	}

	filters := squirrel.Eq{
		"org_id":          req.OrgId,
		"object_type":     req.ObjectType,
		"object_id":       req.ObjectId,
		"annotation_type": models.EntityAnnotationRiskTag,
		"deleted_at":      nil,
		"payload->>'tag'": tag,
	}

	query := NewQueryBuilder().
		Select(dbmodels.EntityAnnotationColumns...).
		From(dbmodels.TABLE_ENTITY_ANNOTATIONS).
		Where(filters).
		Limit(1)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptEntityAnnotation)
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

// FindEntityAnnotationsWithRiskTags finds risk tag annotations matching the filter.
// This is used by MonitoringListCheck rule evaluation.
func (repo *MarbleDbRepository) FindEntityAnnotationsWithRiskTags(
	ctx context.Context,
	exec Executor,
	filter models.EntityAnnotationRiskTagsFilter,
) ([]models.EntityAnnotation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if len(filter.ObjectIds) == 0 {
		return nil, errors.Wrap(models.BadParameterError, "object IDs filter cannot be empty")
	}

	query := NewQueryBuilder().
		Select(dbmodels.EntityAnnotationColumns...).
		From(dbmodels.TABLE_ENTITY_ANNOTATIONS).
		Where(squirrel.Eq{
			"org_id":          filter.OrgId,
			"object_type":     filter.ObjectType,
			"annotation_type": models.EntityAnnotationRiskTag.String(),
			"deleted_at":      nil,
		}).
		Where("object_id = ANY(?)", filter.ObjectIds)

	if len(filter.Tags) > 0 {
		tagStrings := make([]string, len(filter.Tags))
		for i, t := range filter.Tags {
			tagStrings[i] = string(t)
		}
		query = query.Where("payload->>'tag' = ANY(?)", tagStrings)
	}

	query = query.Limit(1)

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptEntityAnnotation)
}

// FindEntityAnnotationsWithRiskTags finds risk tag annotations matching the filter.
// This is used by RecordHasTags rule evaluation.
func (repo *MarbleDbRepository) FindEntityAnnotationsWithTags(
	ctx context.Context,
	exec Executor,
	filter models.EntityAnnotationTagsFilter,
) ([]models.EntityAnnotation, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if len(filter.ObjectIds) == 0 {
		return nil, errors.Wrap(models.BadParameterError, "object IDs filter cannot be empty")
	}

	query := NewQueryBuilder().
		Select(dbmodels.EntityAnnotationColumns...).
		From(dbmodels.TABLE_ENTITY_ANNOTATIONS).
		Where(squirrel.Eq{
			"org_id":          filter.OrgId,
			"object_type":     filter.ObjectType,
			"annotation_type": models.EntityAnnotationTag.String(),
			"deleted_at":      nil,
		}).
		Where("object_id = ANY(?)", filter.ObjectIds)

	if len(filter.Tags) > 0 {
		tagStrings := make([]string, len(filter.Tags))
		for i, t := range filter.Tags {
			tagStrings[i] = t.String()
		}
		query = query.Where("payload->>'tag_id' = ANY(?)", tagStrings)
	}

	query = query.Limit(1)

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptEntityAnnotation)
}
