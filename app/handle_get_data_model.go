package app

import (
	"marble/marble-backend/app/data_model"
)

func (app *App) GetDataModel(orgID string) (data_model.DataModel, error) {
	dataModel, err := app.repository.GetDataModel(orgID)
	if err != nil {
		return data_model.DataModel{}, err
	}
	return dataModel, nil
}
