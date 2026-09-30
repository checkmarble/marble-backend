package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

// ReadCaseDecisionEntityRefs uses the pivot resolver without annotations or related objects.
func (usecase IngestedDataReaderUsecase) ReadCaseDecisionEntityRefs(ctx context.Context, orgId uuid.UUID, values []models.PivotDataWithCount) ([]models.CaseEntityRef, error) {
	objects, err := usecase.readPivotObjectsFromValues(ctx, orgId, values, false)
	if err != nil {
		return nil, err
	}
	refs := make([]models.CaseEntityRef, 0, len(objects))
	for _, object := range objects {
		if object.PivotType == models.PivotTypeObject && object.PivotObjectName != "" && object.PivotObjectId != "" {
			refs = append(refs, models.CaseEntityRef{TableName: object.PivotObjectName, ObjectId: object.PivotObjectId})
		}
	}
	return refs, nil
}
