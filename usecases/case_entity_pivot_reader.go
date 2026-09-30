package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

// ReadCaseDecisionEntityRefs resolves canonical identities without annotations or related data.
// Non-object pivots and historical definitions without a resolvable table/field stay on pivot_objects.
func (usecase IngestedDataReaderUsecase) ReadCaseDecisionEntityRefs(ctx context.Context, orgId uuid.UUID, values []models.PivotDataWithCount) ([]models.CaseEntityRef, error) {
	refs := make([]models.CaseEntityRef, 0)
	if len(values) == 0 {
		return refs, nil
	}
	dm, err := usecase.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{}, true)
	if err != nil {
		return nil, err
	}
	pivots, err := usecase.repository.ListPivots(ctx, usecase.executorFactory.NewExecutor(), orgId, nil, true, true)
	if err != nil {
		return nil, err
	}
	definitions := make(map[string]models.Pivot)
	needsUnique := make(map[string]bool)
	fields, links, tables := dm.AllFieldsAsMap(), dm.AllLinksAsMap(), dm.AllTablesAsMap()
	for _, meta := range pivots {
		if len(meta.PathLinkIds) == 0 && meta.FieldId == nil {
			continue
		}
		valid := true
		for _, id := range meta.PathLinkIds {
			if _, ok := links[id]; !ok {
				valid = false
			}
		}
		if !valid {
			continue
		}
		pivot := meta.Enrich(dm)
		field, ok := fields[pivot.FieldId]
		if !ok || pivot.PivotTable == "" || field.Name == "" {
			continue
		}
		if _, ok := tables[pivot.PivotTableId]; !ok {
			continue
		}
		needsUnique[meta.Id.String()] = len(meta.PathLinkIds) == 0 && field.Name != "object_id"
		definitions[meta.Id.String()] = pivot
	}
	// Collect object_id identities before any ClientDB call, so outages retain them all.
	unresolved := make([]models.PivotDataWithCount, 0)
	for _, value := range values {
		pivot, ok := definitions[value.PivotId]
		if !ok {
			continue
		}
		if pivot.Field == "object_id" {
			if value.PivotValue != "" {
				refs = append(refs, models.CaseEntityRef{TableName: pivot.PivotTable, ObjectId: value.PivotValue})
			}
		} else {
			unresolved = append(unresolved, value)
		}
	}

	if len(unresolved) == 0 {
		return refs, nil
	}
	// Uniqueness introspection reads ClientDB, so it follows collection of every
	// already-known object_id identity and runs only for direct field pivots.
	requiresUniqueLookup := false
	for _, value := range unresolved {
		if needsUnique[value.PivotId] {
			requiresUniqueLookup = true
			break
		}
	}
	if requiresUniqueLookup {
		enriched, err := usecase.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{IncludeUnicityConstraints: true}, true)
		if err != nil {
			return refs, err
		}
		uniqueFields := enriched.AllFieldsAsMap()
		filtered := make([]models.PivotDataWithCount, 0, len(unresolved))
		for _, value := range unresolved {
			if !needsUnique[value.PivotId] || uniqueFields[definitions[value.PivotId].FieldId].UnicityConstraint == models.ActiveUniqueConstraint {
				filtered = append(filtered, value)
			}
		}
		unresolved = filtered
	}
	if len(unresolved) == 0 {
		return refs, nil
	}
	exec, err := usecase.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return refs, err
	}
	for _, value := range unresolved {
		pivot := definitions[value.PivotId]
		objects, err := usecase.clientDbRepository.QueryIngestedObjectByUniqueField(ctx, exec, dm.Tables[pivot.PivotTable], value.PivotValue, pivot.Field)
		if err != nil {
			return refs, repositories.ClientDatabaseError{Err: err}
		}
		if len(objects) != 1 {
			continue
		}
		id, ok := objects[0].Data["object_id"].(string)
		if ok && id != "" {
			refs = append(refs, models.CaseEntityRef{TableName: pivot.PivotTable, ObjectId: id})
		}
	}
	return refs, nil
}
