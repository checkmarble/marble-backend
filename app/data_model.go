package app

import (
	"context"
	"marble/marble-backend/models"
)

func (app *App) GetDataModel(ctx context.Context, orgID string) (models.DataModel, error) {
	dataModel, err := app.repository.GetDataModel(ctx, orgID)
	if err != nil {
		return models.DataModel{}, err
	}
	return dataModel, nil
}
