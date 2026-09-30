package usecases

import (
	"context"
	"slices"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

func (usecase *CaseUseCase) assembleCaseEntities(ctx context.Context, orgId uuid.UUID, manual []models.CaseManualEntity, values []models.PivotDataWithCount) ([]models.CaseEntity, error) {
	refs, err := usecase.ingestedDataReader.ReadCaseDecisionEntityRefs(ctx, orgId, values)
	if err != nil {
		return nil, err
	}
	entities := mergeCaseEntities(manual, refs, nil)
	allRefs := make([]models.CaseEntityRef, len(entities))
	for i, e := range entities {
		allRefs[i] = e.CaseEntityRef
	}
	data, err := usecase.ingestedDataReader.ReadCaseEntityObjects(ctx, orgId, allRefs)
	if err != nil {
		return nil, err
	}
	return mergeCaseEntities(manual, refs, data), nil
}

func mergeCaseEntities(manual []models.CaseManualEntity, decisions []models.CaseEntityRef, data map[models.CaseEntityRef]models.DataModelObject) []models.CaseEntity {
	merged := make(map[models.CaseEntityRef]models.CaseEntity)
	add := func(ref models.CaseEntityRef, source models.CaseEntitySource) {
		entity := merged[ref]
		entity.CaseEntityRef = ref
		if !slices.Contains(entity.Sources, source) {
			entity.Sources = append(entity.Sources, source)
		}
		entity.Data = data[ref].Data
		merged[ref] = entity
	}
	for _, link := range manual {
		add(link.CaseEntityRef, models.CaseEntitySourceManual)
	}
	for _, ref := range decisions {
		add(ref, models.CaseEntitySourceDecision)
	}
	entities := make([]models.CaseEntity, 0, len(merged))
	for _, entity := range merged {
		entities = append(entities, entity)
	}
	slices.SortFunc(entities, func(a, b models.CaseEntity) int {
		if c := strings.Compare(a.TableName, b.TableName); c != 0 {
			return c
		}
		return strings.Compare(a.ObjectId, b.ObjectId)
	})
	return entities
}
