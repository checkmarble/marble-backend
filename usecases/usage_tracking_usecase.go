package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"
)

type UsageTrackingUsecase struct {
	settings        *tracking.Settings
	enforceSecurity security.EnforceSecurity
}

func (uc UsageTrackingUsecase) SetEnabled(ctx context.Context, enabled bool) error {
	if err := uc.enforceSecurity.Permission(models.ORGANIZATIONS_UPDATE); err != nil {
		return err
	}
	return uc.settings.SetEnabled(ctx, enabled)
}
