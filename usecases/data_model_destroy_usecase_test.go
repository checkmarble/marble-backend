package usecases

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
)

func TestDataModelDestroyUsecase_DeleteLink_foreignLink(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.New()
	exec := new(mocks.Executor)
	executorFactory := new(mocks.ExecutorFactory)
	enforceSecurity := new(mocks.EnforceSecurity)
	dataModelRepository := new(mocks.DataModelRepository)

	executorFactory.On("NewExecutor").Return(exec)
	enforceSecurity.On("OrgId").Return(orgId)
	enforceSecurity.On("WriteDataModel", orgId).Return(nil)
	dataModelRepository.On("GetLinks", ctx, exec, orgId).
		Return([]models.LinkToSingle{{Id: uuid.NewString()}}, nil)

	uc := DataModelDestroyUsecase{
		executorFactory:     executorFactory,
		enforceSecurity:     enforceSecurity,
		dataModelRepository: dataModelRepository,
	}

	_, err := uc.DeleteLink(ctx, false, uuid.NewString())

	assert.ErrorIs(t, err, models.NotFoundError)
	dataModelRepository.AssertExpectations(t)
	dataModelRepository.AssertNotCalled(t, "DeleteDataModelLink")
}

func TestDataModelDestroyUsecase_DeletePivot_foreignPivot(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.New()
	pivotId := uuid.New()
	exec := new(mocks.Executor)
	executorFactory := new(mocks.ExecutorFactory)
	enforceSecurity := new(mocks.EnforceSecurity)
	dataModelRepository := new(mocks.DataModelRepository)

	executorFactory.On("NewExecutor").Return(exec)
	enforceSecurity.On("OrgId").Return(orgId)
	enforceSecurity.On("WriteDataModel", orgId).Return(nil)
	dataModelRepository.On("GetPivot", ctx, exec, pivotId.String()).
		Return(models.PivotMetadata{Id: pivotId, OrganizationId: uuid.New()}, nil)

	uc := DataModelDestroyUsecase{
		executorFactory:     executorFactory,
		enforceSecurity:     enforceSecurity,
		dataModelRepository: dataModelRepository,
	}

	_, err := uc.DeletePivot(ctx, false, pivotId.String())

	assert.ErrorIs(t, err, models.NotFoundError)
	dataModelRepository.AssertExpectations(t)
	dataModelRepository.AssertNotCalled(t, "SoftDeletePivot")
}
