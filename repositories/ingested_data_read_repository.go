package repositories

import (
	"context"
	"marble/marble-backend/models"
)

type IngestedDataReadRepository interface {
	GetDbField(ctx context.Context, readParams models.DbFieldReadParams) (interface{}, error)
}
