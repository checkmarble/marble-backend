package usecases

import (
	"context"
	"slices"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

// readCaseEntities returns only the case's explicit manual links and their customer data.
func (usecase *CaseUseCase) readCaseEntities(ctx context.Context, orgId uuid.UUID, manual []models.CaseManualEntity) ([]models.CaseEntity, error) {
	refs := make([]models.CaseEntityRef, len(manual))
	for i, link := range manual {
		refs[i] = link.CaseEntityRef
	}
	data, err := usecase.caseEntityReader.ReadCaseEntityObjects(ctx, orgId, refs)
	if err != nil {
		return nil, err
	}
	entities := make([]models.CaseEntity, 0, len(manual))
	for _, link := range manual {
		entities = append(entities, models.CaseEntity{
			CaseEntityRef: link.CaseEntityRef,
			Data:          data[link.CaseEntityRef].Data,
		})
	}
	slices.SortFunc(entities, func(a, b models.CaseEntity) int {
		if c := strings.Compare(a.TableName, b.TableName); c != 0 {
			return c
		}
		return strings.Compare(a.ObjectId, b.ObjectId)
	})
	return entities, nil
}
