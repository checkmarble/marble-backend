package mocks

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type WatermarkRepository struct {
	mock.Mock
}

func (w *WatermarkRepository) GetWatermark(ctx context.Context, exec repositories.Executor,
	orgId *uuid.UUID, watermarkType models.WatermarkType,
) (*models.Watermark, error) {
	args := w.Called(ctx, exec, orgId, watermarkType)
	var wm *models.Watermark
	if v := args.Get(0); v != nil {
		wm = v.(*models.Watermark)
	}
	return wm, args.Error(1)
}

func (w *WatermarkRepository) SaveWatermark(ctx context.Context, exec repositories.Executor,
	orgId *uuid.UUID, watermarkType models.WatermarkType, watermarkId *string,
	watermarkTime time.Time, params json.RawMessage,
) error {
	args := w.Called(ctx, exec, orgId, watermarkType, watermarkId, watermarkTime, params)
	return args.Error(0)
}
