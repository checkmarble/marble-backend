package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeFeatureFlagSecurity struct {
	err error
}

func (s fakeFeatureFlagSecurity) ReadFeatureFlags() error  { return s.err }
func (s fakeFeatureFlagSecurity) WriteFeatureFlags() error { return s.err }

type fakeFeatureFlagRepository struct {
	stored map[string]string
}

func (r *fakeFeatureFlagRepository) ListFeatureFlags(ctx context.Context, exec repositories.Executor) (map[string]string, error) {
	return r.stored, nil
}

func (r *fakeFeatureFlagRepository) ReplaceFeatureFlags(ctx context.Context, exec repositories.Transaction, flags map[string]string) error {
	r.stored = flags
	return nil
}

func newFeatureFlagUsecase(securityErr error) (FeatureFlagUsecase, *fakeFeatureFlagRepository) {
	repo := &fakeFeatureFlagRepository{stored: map[string]string{}}

	txFactory := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
	txFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)

	execFactory := &mocks.ExecutorFactory{}
	execFactory.On("NewExecutor").Return(&mocks.Executor{})

	return FeatureFlagUsecase{
		executorFactory:    execFactory,
		transactionFactory: txFactory,
		enforceSecurity:    fakeFeatureFlagSecurity{err: securityErr},
		repository:         repo,
	}, repo
}

func TestReplaceFeatureFlags_Forbidden(t *testing.T) {
	uc, repo := newFeatureFlagUsecase(models.ForbiddenError)

	_, err := uc.ReplaceFeatureFlags(context.Background(), map[string]string{"ASYNC_DECISION_STORAGE": "all"})

	assert.ErrorIs(t, err, models.ForbiddenError)
	assert.Empty(t, repo.stored)
}

func TestReplaceFeatureFlags_UnknownFlag(t *testing.T) {
	uc, _ := newFeatureFlagUsecase(nil)

	_, err := uc.ReplaceFeatureFlags(context.Background(), map[string]string{"UNKNOWN": "all"})

	assert.ErrorIs(t, err, models.BadParameterError)
}

func TestReplaceFeatureFlags_InvalidValue(t *testing.T) {
	uc, _ := newFeatureFlagUsecase(nil)

	_, err := uc.ReplaceFeatureFlags(context.Background(), map[string]string{"ASYNC_DECISION_STORAGE": "all,notauuid"})

	assert.ErrorIs(t, err, models.BadParameterError)
}

func TestReplaceFeatureFlags_Valid(t *testing.T) {
	uc, repo := newFeatureFlagUsecase(nil)
	orgs := pure_utils.NewId().String() + "," + pure_utils.NewId().String()

	flags, err := uc.ReplaceFeatureFlags(context.Background(), map[string]string{"ASYNC_DECISION_STORAGE": orgs})

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"ASYNC_DECISION_STORAGE": orgs}, flags)
	assert.Equal(t, flags, repo.stored)
}

func TestReplaceFeatureFlags_EmptyValueRemovesFlag(t *testing.T) {
	uc, repo := newFeatureFlagUsecase(nil)
	repo.stored = map[string]string{"ASYNC_DECISION_STORAGE": "all"}

	flags, err := uc.ReplaceFeatureFlags(context.Background(), map[string]string{"ASYNC_DECISION_STORAGE": ""})

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"ASYNC_DECISION_STORAGE": ""}, flags)
	assert.Empty(t, repo.stored)
}

func TestListFeatureFlags_IncludesUnsetKnownFlags(t *testing.T) {
	uc, _ := newFeatureFlagUsecase(nil)

	flags, err := uc.ListFeatureFlags(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"ASYNC_DECISION_STORAGE": ""}, flags)
}

func TestListFeatureFlags_OmitsUnknownStoredFlags(t *testing.T) {
	uc, repo := newFeatureFlagUsecase(nil)
	repo.stored = map[string]string{"ASYNC_DECISION_STORAGE": "all", "REMOVED_FLAG": "all"}

	flags, err := uc.ListFeatureFlags(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"ASYNC_DECISION_STORAGE": "all"}, flags)
}
