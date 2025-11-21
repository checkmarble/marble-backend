package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

type ContinuousScreeningUsecase struct {
	mock.Mock
}

func (m *ContinuousScreeningUsecase) GetDataModelTableAndMapping(
	ctx context.Context,
	exec repositories.Executor,
	config models.ContinuousScreeningConfig,
	objectType string,
) (models.Table, models.ContinuousScreeningDataModelMapping, error) {
	args := m.Called(ctx, exec, config, objectType)
	return args.Get(0).(models.Table), args.Get(1).(models.ContinuousScreeningDataModelMapping), args.Error(2)
}

func (m *ContinuousScreeningUsecase) GetIngestedObject(
	ctx context.Context,
	clientDbExec repositories.Executor,
	table models.Table,
	objectId string,
) (models.DataModelObject, uuid.UUID, error) {
	args := m.Called(ctx, clientDbExec, table, objectId)
	return args.Get(0).(models.DataModelObject), args.Get(1).(uuid.UUID), args.Error(2)
}

func (m *ContinuousScreeningUsecase) DoScreening(
	ctx context.Context,
	ingestedObject models.DataModelObject,
	mapping models.ContinuousScreeningDataModelMapping,
	config models.ContinuousScreeningConfig,
) (models.ScreeningWithMatches, error) {
	args := m.Called(ctx, ingestedObject, mapping, config)
	return args.Get(0).(models.ScreeningWithMatches), args.Error(1)
}

func (m *ContinuousScreeningUsecase) HandleCaseCreation(
	ctx context.Context,
	tx repositories.Transaction,
	config models.ContinuousScreeningConfig,
	objectId string,
	continuousScreeningWithMatches models.ContinuousScreeningWithMatches,
) error {
	args := m.Called(ctx, tx, config, objectId, continuousScreeningWithMatches)
	return args.Error(0)
}
