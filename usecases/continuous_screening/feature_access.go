package continuous_screening

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func (uc *ContinuousScreeningUsecase) CheckFeatureAccess(ctx context.Context, orgId uuid.UUID) error {
	features, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, orgId.String(), nil)
	if err != nil {
		return errors.Wrap(err, "could not check feature access")
	}

	if !features.ContinuousScreening.IsAllowed() {
		return errors.Wrap(models.ForbiddenError, "continuous screening feature is not allowed")
	}

	return nil
}
