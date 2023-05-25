package repositories

import (
	"context"
	"marble/marble-backend/models"
)

type DataModelRepository interface {
	GetDataModel(ctx context.Context, organizationID string) (models.DataModel, error)
}
