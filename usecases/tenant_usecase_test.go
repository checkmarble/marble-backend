package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type tenantMergeSecurityMock struct {
	err error
}

func (s tenantMergeSecurityMock) Permission(models.Permission) error { return s.err }
func (tenantMergeSecurityMock) ReadOrganization(uuid.UUID) error     { return nil }
func (tenantMergeSecurityMock) Permissions([]models.Permission) error {
	return nil
}
func (tenantMergeSecurityMock) OrgId() uuid.UUID  { return uuid.Nil }
func (tenantMergeSecurityMock) UserId() *string   { return nil }
func (tenantMergeSecurityMock) ApiKeyId() *string { return nil }

func TestTenantUsecaseMerge(t *testing.T) {
	target := uuid.New()
	source := uuid.New()
	newName := "merged tenant"
	input := TenantMergeInput{
		TargetTenantId:  target,
		SourceTenantIds: []uuid.UUID{source},
	}

	tests := []struct {
		name          string
		input         TenantMergeInput
		securityError error
		wantError     error
		setup         func(*mocks.TransactionFactory, *mocks.TenantRepository, *mocks.GrantRepository, *mocks.OrganizationRepository)
	}{
		{
			name:          "permission denied",
			input:         input,
			securityError: models.ForbiddenError,
			wantError:     models.ForbiddenError,
		},
		{
			name:      "missing sources",
			input:     TenantMergeInput{TargetTenantId: target},
			wantError: models.BadParameterError,
		},
		{
			name: "target is a source",
			input: TenantMergeInput{
				TargetTenantId:  target,
				SourceTenantIds: []uuid.UUID{target},
			},
			wantError: models.BadParameterError,
		},
		{
			name: "duplicate source",
			input: TenantMergeInput{
				TargetTenantId:  target,
				SourceTenantIds: []uuid.UUID{source, source},
			},
			wantError: models.BadParameterError,
		},
		{
			name:  "merge succeeds",
			input: input,
			setup: func(transactionFactory *mocks.TransactionFactory, tenantRepository *mocks.TenantRepository, grantRepository *mocks.GrantRepository, organizationRepository *mocks.OrganizationRepository) {
				transactionFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
				tenantRepository.On("LockAndValidateMerge", mock.Anything, mock.Anything, target, []uuid.UUID{source}).Return(nil)
				grantRepository.On("ReassignTenantGrants", mock.Anything, mock.Anything, target, []uuid.UUID{source}).Return(nil)
				organizationRepository.On("ReassignOrganizationsToTenant", mock.Anything, mock.Anything, target, []uuid.UUID{source}).Return(nil)
				tenantRepository.On("SoftDelete", mock.Anything, mock.Anything, []uuid.UUID{source}).Return(nil)
			},
		},
		{
			name: "merge succeeds with new name",
			input: TenantMergeInput{
				TargetTenantId:  target,
				SourceTenantIds: []uuid.UUID{source},
				NewName:         &newName,
			},
			setup: func(transactionFactory *mocks.TransactionFactory, tenantRepository *mocks.TenantRepository, grantRepository *mocks.GrantRepository, organizationRepository *mocks.OrganizationRepository) {
				transactionFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
				tenantRepository.On("LockAndValidateMerge", mock.Anything, mock.Anything, target, []uuid.UUID{source}).Return(nil)
				grantRepository.On("ReassignTenantGrants", mock.Anything, mock.Anything, target, []uuid.UUID{source}).Return(nil)
				organizationRepository.On("ReassignOrganizationsToTenant", mock.Anything, mock.Anything, target, []uuid.UUID{source}).Return(nil)
				tenantRepository.On("SoftDelete", mock.Anything, mock.Anything, []uuid.UUID{source}).Return(nil)
				tenantRepository.On("Rename", mock.Anything, mock.Anything, target, newName).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transactionFactory := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
			executorFactory := &mocks.ExecutorFactory{}
			tenantRepository := &mocks.TenantRepository{}
			grantRepository := &mocks.GrantRepository{}
			organizationRepository := &mocks.OrganizationRepository{}
			if tt.setup != nil {
				tt.setup(transactionFactory, tenantRepository, grantRepository, organizationRepository)
			}

			usecase := NewTenantUsecase(
				tenantMergeSecurityMock{err: tt.securityError},
				transactionFactory,
				executorFactory,
				tenantRepository,
				grantRepository,
				organizationRepository,
			)
			err := usecase.Merge(context.Background(), tt.input)
			if tt.wantError != nil {
				require.ErrorIs(t, err, tt.wantError)
				return
			}
			require.NoError(t, err)
			tenantRepository.AssertExpectations(t)
			grantRepository.AssertExpectations(t)
			organizationRepository.AssertExpectations(t)
			transactionFactory.AssertExpectations(t)
		})
	}
}

func TestTenantUsecaseList(t *testing.T) {
	executor := &mocks.Executor{}
	executorFactory := &mocks.ExecutorFactory{}
	tenantRepository := &mocks.TenantRepository{}
	want := []models.Tenant{{Id: uuid.New(), Name: "Tenant"}}
	executorFactory.On("NewExecutor").Return(executor)
	tenantRepository.On("ListActive", mock.Anything, executor).Return(want, nil)

	usecase := NewTenantUsecase(
		tenantMergeSecurityMock{},
		&mocks.TransactionFactory{},
		executorFactory,
		tenantRepository,
		&mocks.GrantRepository{},
		&mocks.OrganizationRepository{},
	)
	got, err := usecase.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, want, got)
	tenantRepository.AssertExpectations(t)
	executorFactory.AssertExpectations(t)
}

func TestTenantUsecaseUpdateName(t *testing.T) {
	tenantId := uuid.New()
	executorFactory := &mocks.ExecutorFactory{}
	tenantRepository := &mocks.TenantRepository{}
	transactionFactory := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
	transactionFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
	tenantRepository.On("LockAndValidateUpdate", mock.Anything, mock.Anything, tenantId).Return(nil)
	tenantRepository.On("Rename", mock.Anything, mock.Anything, tenantId, "Renamed").Return(nil)

	usecase := NewTenantUsecase(
		tenantMergeSecurityMock{},
		transactionFactory,
		executorFactory,
		tenantRepository,
		&mocks.GrantRepository{},
		&mocks.OrganizationRepository{},
	)
	err := usecase.UpdateName(context.Background(), tenantId, "Renamed")
	require.NoError(t, err)
	tenantRepository.AssertExpectations(t)
	transactionFactory.AssertExpectations(t)
}

func TestTenantUsecaseUpdateNameRejectsBlankName(t *testing.T) {
	usecase := NewTenantUsecase(
		tenantMergeSecurityMock{},
		&mocks.TransactionFactory{},
		&mocks.ExecutorFactory{},
		&mocks.TenantRepository{},
		&mocks.GrantRepository{},
		&mocks.OrganizationRepository{},
	)

	err := usecase.UpdateName(context.Background(), uuid.New(), "   ")
	require.ErrorIs(t, err, models.BadParameterError)
}
