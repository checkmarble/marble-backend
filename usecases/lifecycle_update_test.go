package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/sosodev/duration"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLifecycleUpdateEnqueuesProvisioning(t *testing.T) {
	d := &duration.Duration{Days: 31}
	cases := []struct {
		name   string
		policy *models.TableLifecycle
	}{
		{"omitted", nil},
		{"disabled", &models.TableLifecycle{}},
		{"active", &models.TableLifecycle{Enabled: true, DeleteActiveRowsAfter: d}},
		{"stale", &models.TableLifecycle{Enabled: true, DeleteStaleRowsAfter: d}},
		{"both", &models.TableLifecycle{Enabled: true, DeleteActiveRowsAfter: d, DeleteStaleRowsAfter: d}},
		{"changed duration", &models.TableLifecycle{Enabled: true, DeleteActiveRowsAfter: &duration.Duration{Days: 60}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			org := uuid.New()
			tx := &mocks.Transaction{}
			factory := &mocks.TransactionFactory{TxMock: tx}
			factory.On("Transaction", ctx, mock.Anything).Return(nil)
			data := &mocks.DataModelRepository{}
			table := models.TableMetadata{Name: "transactions", OrganizationID: org}
			data.On("GetDataModelTable", ctx, tx, "table-id").Return(table, nil)
			data.On("UpdateDataModelTable", ctx, tx, "table-id", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, tc.policy).Return(nil)
			stop := errors.New("stop after enqueue")
			data.On("GetLinks", ctx, tx, org).Return([]models.LinkToSingle(nil), stop)
			security := &mocks.EnforceSecurity{}
			security.On("WriteDataModel", org).Return(nil)
			queue := &mocks.TaskQueueRepository{}
			if tc.policy != nil {
				queue.On("EnqueueLifecycleIndexProvisionTask", ctx, tx, org, "table-id").Return(nil).Once()
			}
			uc := usecase{transactionFactory: factory, dataModelRepository: data, enforceSecurity: security, taskQueueRepository: queue}
			_, err := uc.UpdateDataModelTableComposite(ctx, "table-id", models.UpdateTableCompositeInput{Lifecycle: tc.policy})
			require.ErrorIs(t, err, stop)
			queue.AssertExpectations(t)
			if tc.policy == nil {
				queue.AssertNotCalled(t, "EnqueueLifecycleIndexProvisionTask", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
			queue.AssertNotCalled(t, "EnqueueDeleteIndexByNameTask", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestLifecycleUpdateSecurityFailureDoesNotEnqueue(t *testing.T) {
	ctx := context.Background()
	org := uuid.New()
	tx := &mocks.Transaction{}
	factory := &mocks.TransactionFactory{TxMock: tx}
	factory.On("Transaction", ctx, mock.Anything).Return(nil)
	data := &mocks.DataModelRepository{}
	data.On("GetDataModelTable", ctx, tx, "table-id").Return(models.TableMetadata{OrganizationID: org}, nil)
	security := &mocks.EnforceSecurity{}
	denied := errors.New("denied")
	security.On("WriteDataModel", org).Return(denied)
	queue := &mocks.TaskQueueRepository{}
	uc := usecase{transactionFactory: factory, dataModelRepository: data, enforceSecurity: security, taskQueueRepository: queue}
	_, err := uc.UpdateDataModelTableComposite(ctx, "table-id", models.UpdateTableCompositeInput{Lifecycle: &models.TableLifecycle{Enabled: true}})
	require.ErrorIs(t, err, denied)
	queue.AssertNotCalled(t, "EnqueueLifecycleIndexProvisionTask", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
