package usecases

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
)

type mockMetricsIngestionRepository struct {
	mock.Mock
}

func (m *mockMetricsIngestionRepository) SendMetrics(ctx context.Context, collection models.MetricsCollection) error {
	args := m.Called(ctx, collection)
	return args.Error(0)
}

type mockPublicLicenseRepository struct {
	mock.Mock
}

func (m *mockPublicLicenseRepository) GetLicenseByKey(ctx context.Context,
	exec repositories.Executor, licenseKey string,
) (models.License, error) {
	args := m.Called(ctx, exec, licenseKey)
	return args.Get(0).(models.License), args.Error(1)
}

type MetricsIngestionUsecaseTestSuite struct {
	suite.Suite
	metricRepository  *mockMetricsIngestionRepository
	licenseRepository *mockPublicLicenseRepository
	executorFactory   *mocks.ExecutorFactory
	executor          *mocks.Executor

	licenseKey      string
	repositoryError error
	licenseError    error
}

func (suite *MetricsIngestionUsecaseTestSuite) SetupTest() {
	suite.metricRepository = new(mockMetricsIngestionRepository)
	suite.licenseRepository = new(mockPublicLicenseRepository)
	suite.executorFactory = new(mocks.ExecutorFactory)
	suite.executor = new(mocks.Executor)

	suite.licenseKey = "test-license-key"
	suite.repositoryError = errors.New("repository error")
	suite.licenseError = errors.New("license error")
}

func (suite *MetricsIngestionUsecaseTestSuite) makeUsecase() *MetricsIngestionUsecase {
	return &MetricsIngestionUsecase{
		metricRepository:  suite.metricRepository,
		licenseRepository: suite.licenseRepository,
		executorFactory:   suite.executorFactory,
	}
}

func (suite *MetricsIngestionUsecaseTestSuite) Test_IngestMetrics_WithValidLicense() {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("test"))
	collection := models.MetricsCollection{
		LicenseKey: &suite.licenseKey,
		Metrics:    []models.MetricData{},
	}

	suite.executorFactory.On("NewExecutor").Return(suite.executor)
	suite.licenseRepository.On("GetLicenseByKey", ctx, suite.executor, suite.licenseKey).
		Return(models.License{}, nil)
	suite.metricRepository.On("SendMetrics", ctx, collection).Return(nil)

	err := suite.makeUsecase().IngestMetrics(ctx, collection)

	suite.NoError(err)
}

func (suite *MetricsIngestionUsecaseTestSuite) Test_IngestMetrics_WithInvalidLicense() {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("test"))
	collection := models.MetricsCollection{
		LicenseKey: &suite.licenseKey,
		Metrics:    []models.MetricData{},
	}

	suite.executorFactory.On("NewExecutor").Return(suite.executor)
	suite.licenseRepository.On("GetLicenseByKey", ctx, suite.executor, suite.licenseKey).
		Return(models.License{}, models.NotFoundError)

	err := suite.makeUsecase().IngestMetrics(ctx, collection)

	suite.ErrorIs(err, models.UnAuthorizedError)
	// Verify again after the call that SendMetrics was never called
	suite.metricRepository.AssertNotCalled(suite.T(), "SendMetrics")
}

func (suite *MetricsIngestionUsecaseTestSuite) Test_IngestMetrics_WithoutLicense() {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("test"))
	collection := models.MetricsCollection{
		LicenseKey: nil,
		Metrics:    []models.MetricData{},
	}

	suite.metricRepository.On("SendMetrics", ctx, collection).Return(nil)

	err := suite.makeUsecase().IngestMetrics(ctx, collection)

	suite.NoError(err)
	suite.licenseRepository.AssertNotCalled(suite.T(), "GetLicenseByKey")
}

func TestMetricsIngestionUsecase(t *testing.T) {
	suite.Run(t, new(MetricsIngestionUsecaseTestSuite))
}
